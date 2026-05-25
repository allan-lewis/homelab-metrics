package metrics

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
}

type nixOSInfo struct {
	GenerationCount            int
	CurrentGeneration          int
	CurrentGenerationTimestamp int64
	LastSwitchTimestamp         int64
	BootedIsCurrent            bool
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
			"Whether the booted NixOS system resolves to the current NixOS system.",
			nil,
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
}

func (c *NixOSCollector) Collect(ch chan<- prometheus.Metric) {
	info, err := collectNixOSInfo()
	if err != nil {
		log.Printf("Failed to collect NixOS metrics: %v", err)
		return
	}

	log.Printf(
		"NixOS metrics collected: generation_count=%d current_generation=%d current_generation_timestamp=%d last_switch_timestamp=%d booted_is_current=%t",
		info.GenerationCount,
		info.CurrentGeneration,
		info.CurrentGenerationTimestamp,
		info.LastSwitchTimestamp,
		info.BootedIsCurrent,
	)

	ch <- prometheus.MustNewConstMetric(
		c.genCount,
		prometheus.GaugeValue,
		float64(info.GenerationCount),
	)

	ch <- prometheus.MustNewConstMetric(
		c.currentGen,
		prometheus.GaugeValue,
		float64(info.CurrentGeneration),
	)

	ch <- prometheus.MustNewConstMetric(
		c.currentGenTime,
		prometheus.GaugeValue,
		float64(info.CurrentGenerationTimestamp),
	)

	ch <- prometheus.MustNewConstMetric(
		c.lastSwitchTime,
		prometheus.GaugeValue,
		float64(info.LastSwitchTimestamp),
	)

	ch <- prometheus.MustNewConstMetric(
		c.bootedCurrent,
		prometheus.GaugeValue,
		boolFloat(info.BootedIsCurrent),
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
			log.Printf("Ignoring unexpected NixOS system generation link %q: %v", name, err)
			continue
		}

		generations = append(generations, generation)
	}

	if len(generations) == 0 {
		return nil, fmt.Errorf("no NixOS system generation links found in %s", profilesDir)
	}

	sort.Ints(generations)

	currentPath, err := filepath.EvalSymlinks(currentSystem)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", currentSystem, err)
	}

	bootedPath, err := filepath.EvalSymlinks(bootedSystem)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", bootedSystem, err)
	}

	bootedIsCurrent := currentPath == bootedPath

	var currentGeneration int
	var currentGenerationTimestamp int64

	for _, generation := range generations {
		linkPath := fmt.Sprintf("%s/system-%d-link", profilesDir, generation)

		targetPath, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			log.Printf("Failed to resolve NixOS generation link %s: %v", linkPath, err)
			continue
		}

		if targetPath != currentPath {
			continue
		}

		linkInfo, err := os.Lstat(linkPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat current NixOS generation link %s: %w", linkPath, err)
		}

		currentGeneration = generation
		currentGenerationTimestamp = linkInfo.ModTime().Unix()
		break
	}

	if currentGeneration == 0 {
		return nil, fmt.Errorf("failed to match %s resolved path %s to a system generation link", currentSystem, currentPath)
	}

	systemProfileInfo, err := os.Lstat(systemProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat NixOS system profile symlink %s: %w", systemProfile, err)
	}

	return &nixOSInfo{
		GenerationCount:            len(generations),
		CurrentGeneration:          currentGeneration,
		CurrentGenerationTimestamp: currentGenerationTimestamp,
		LastSwitchTimestamp:         systemProfileInfo.ModTime().Unix(),
		BootedIsCurrent:            bootedIsCurrent,
	}, nil
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}

	return 0
}
