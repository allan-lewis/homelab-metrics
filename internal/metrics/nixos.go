package metrics

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type NixOSCollector struct {
	genCount        *prometheus.Desc
	currentGen     *prometheus.Desc
	currentGenTime *prometheus.Desc
	lastSwitchTime *prometheus.Desc
	bootedCurrent  *prometheus.Desc
	info           *prometheus.Desc
}

type nixOSInfo struct {
	GenerationCount            int
	CurrentGeneration          int
	LatestGeneration           int
	BootedGeneration           int
	CurrentGenerationTimestamp int64
	LastSwitchTimestamp         int64
	BootedIsCurrent            bool
	CurrentSystem              string
	BootedSystem               string
	CurrentVersion             string
	BootedVersion              string
}

func NewNixOSCollector() *NixOSCollector {
	log.Println("Initializing NixOS metrics collector")

	return &NixOSCollector{
		genCount: prometheus.NewDesc(
			"nixos_system_generation_count",
			"Number of NixOS system generations.",
			nil,
			nil,
		),
		currentGen: prometheus.NewDesc(
			"nixos_system_generation_current",
			"Current NixOS system generation number.",
			nil,
			nil,
		),
		currentGenTime: prometheus.NewDesc(
			"nixos_system_generation_current_timestamp_seconds",
			"Unix timestamp of the current NixOS generation symlink.",
			nil,
			nil,
		),
		lastSwitchTime: prometheus.NewDesc(
			"nixos_system_last_switch_timestamp_seconds",
			"Unix timestamp of the last NixOS system profile switch.",
			nil,
			nil,
		),
		bootedCurrent: prometheus.NewDesc(
			"nixos_system_booted_is_current",
			"Whether /run/booted-system points to the same Nix store system as /run/current-system.",
			nil,
			nil,
		),
		info: prometheus.NewDesc(
			"nixos_system_info",
			"NixOS system information. Value is always 1; details are exposed as labels.",
			[]string{
					"current_system",
					"booted_system",
					"current_generation",
					"booted_generation",
					"current_version",
					"booted_version",
					"booted_is_current",
			},
			nil,
		),
	}
}

func (c *NixOSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.genCount
	ch <- c.currentGen
	ch <- c.currentGenTime
	ch <- c.lastSwitchTime
	ch <- c.bootedCurrent
	ch <- c.info
}

func (c *NixOSCollector) Collect(ch chan<- prometheus.Metric) {
	info, err := collectNixOSInfo()
	if err != nil {
		log.Printf("Failed to collect NixOS metrics: %v", err)
		return
	}

	log.Printf(
		"NixOS metrics collected: generation_count=%d current_generation=%d latest_generation=%d booted_generation=%d current_generation_timestamp=%d last_switch_timestamp=%d booted_is_current=%t current_system=%s booted_system=%s current_version=%s booted_version=%s",
		info.GenerationCount,
		info.CurrentGeneration,
		info.LatestGeneration,
		info.BootedGeneration,
		info.CurrentGenerationTimestamp,
		info.LastSwitchTimestamp,
		info.BootedIsCurrent,
		info.CurrentSystem,
		info.BootedSystem,
		info.CurrentVersion,
		info.BootedVersion,
	)

	ch <- prometheus.MustNewConstMetric(c.genCount, prometheus.GaugeValue, float64(info.GenerationCount))
	ch <- prometheus.MustNewConstMetric(c.currentGen, prometheus.GaugeValue, float64(info.CurrentGeneration))
	ch <- prometheus.MustNewConstMetric(c.currentGenTime, prometheus.GaugeValue, float64(info.CurrentGenerationTimestamp))
	ch <- prometheus.MustNewConstMetric(c.lastSwitchTime, prometheus.GaugeValue, float64(info.LastSwitchTimestamp))
	ch <- prometheus.MustNewConstMetric(c.bootedCurrent, prometheus.GaugeValue, boolFloat(info.BootedIsCurrent))

	ch <- prometheus.MustNewConstMetric(
			c.info,
			prometheus.GaugeValue,
			1,
			info.CurrentSystem,
			info.BootedSystem,
			strconv.Itoa(info.CurrentGeneration),
			strconv.Itoa(info.BootedGeneration),
			info.CurrentVersion,
			info.BootedVersion,
			strconv.FormatBool(info.BootedIsCurrent),
	)
}

func collectNixOSInfo() (*nixOSInfo, error) {
	const profilesDir = "/nix/var/nix/profiles"
	const systemProfile = "/nix/var/nix/profiles/system"
	const currentSystem = "/run/current-system"
	const bootedSystem = "/run/booted-system"

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read NixOS profiles directory %s: %w", profilesDir, err)
	}

	generations := make([]int, 0)

	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasPrefix(name, "system-") || !strings.HasSuffix(name, "-link") {
			continue
		}

		rawGeneration := strings.TrimSuffix(strings.TrimPrefix(name, "system-"), "-link")

		generation, err := strconv.Atoi(rawGeneration)
		if err != nil {
			log.Printf("Ignoring unexpected NixOS generation link %q: %v", name, err)
			continue
		}

		generations = append(generations, generation)
	}

	if len(generations) == 0 {
		return nil, fmt.Errorf("no NixOS system generation links found in %s", profilesDir)
	}

	sort.Ints(generations)
	latestGeneration := generations[len(generations)-1]

	currentPath, err := readNixStoreSymlink(currentSystem)
	if err != nil {
		return nil, err
	}

	bootedPath, err := readNixStoreSymlink(bootedSystem)
	if err != nil {
		return nil, err
	}

	bootedIsCurrent := currentPath == bootedPath

	var currentGeneration int
	var bootedGeneration int
	var currentGenerationTimestamp int64

	for _, generation := range generations {
		linkPath := fmt.Sprintf("%s/system-%d-link", profilesDir, generation)

		targetPath, err := readNixStoreSymlink(linkPath)
		if err != nil {
			log.Printf("Skipping NixOS generation link %s: %v", linkPath, err)
			continue
		}

		if targetPath == bootedPath {
			bootedGeneration = generation
		}

		if targetPath != currentPath {
			continue
		}

		linkSymlinkInfo, err := os.Lstat(linkPath)
		if err != nil {
			return nil, fmt.Errorf("failed to lstat current NixOS generation link %s: %w", linkPath, err)
		}

		currentGeneration = generation
		currentGenerationTimestamp = linkSymlinkInfo.ModTime().Unix()
	}

	if currentGeneration == 0 {
		return nil, fmt.Errorf("failed to match %s target %s to a system generation link", currentSystem, currentPath)
	}

	if bootedGeneration == 0 {
		return nil, fmt.Errorf("failed to match %s target %s to a system generation link", bootedSystem, bootedPath)
	}

	systemProfileInfo, err := os.Lstat(systemProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to lstat NixOS system profile symlink %s: %w", systemProfile, err)
	}

	log.Printf(
		"NixOS system targets: current=%s booted=%s booted_is_current=%t",
		currentPath,
		bootedPath,
		bootedIsCurrent,
	)

	return &nixOSInfo{
		GenerationCount:            len(generations),
		CurrentGeneration:          currentGeneration,
		LatestGeneration:           latestGeneration,
		BootedGeneration:           bootedGeneration,
		CurrentGenerationTimestamp: currentGenerationTimestamp,
		LastSwitchTimestamp:         systemProfileInfo.ModTime().Unix(),
		BootedIsCurrent:            bootedIsCurrent,
		CurrentSystem:              nixOSSystemName(currentPath),
		BootedSystem:               nixOSSystemName(bootedPath),
		CurrentVersion:             nixOSVersion(currentPath),
		BootedVersion:              nixOSVersion(bootedPath),
	}, nil
}

func readNixStoreSymlink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink %s: %w", path, err)
	}

	if !strings.HasPrefix(target, "/nix/store/") {
		return "", fmt.Errorf("symlink %s target is not a Nix store path: %s", path, target)
	}

	return target, nil
}

func nixOSSystemName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}

	return path
}

func nixOSVersion(path string) string {
	base := nixOSSystemName(path)

	const marker = "-nixos-system-"
	idx := strings.Index(base, marker)
	if idx < 0 {
		return "unknown"
	}

	raw := base[idx+len(marker):]
	parts := strings.Split(raw, "-")

	if len(parts) < 2 {
		return "unknown"
	}

	versionParts := strings.Split(parts[1], ".")
	if len(versionParts) < 2 {
		return "unknown"
	}

	return versionParts[0] + "." + versionParts[1]
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}

	return 0
}
