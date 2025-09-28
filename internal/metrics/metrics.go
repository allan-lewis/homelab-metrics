package metrics

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	publicIPMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "homelab_public_ip_numeric",
		Help: "Numeric representation of the public IP address",
	})

	publicIPLastUpdate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "homelab_public_ip_last_success_unix",
		Help: "Unix timestamp of the last successful public IP metric update",
	})
)

func init() {
	prometheus.MustRegister(publicIPMetric)
	prometheus.MustRegister(publicIPLastUpdate)

	go func() {
		updatePublicIPMetric()
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			updatePublicIPMetric()
		}
	}()
}

func Handler() http.Handler {
	r := chi.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	return r
}

func updatePublicIPMetric() {
	ip, err := getPublicIP()
	if err != nil {
		return
	}

	ipNum, err := convertIPv4ToUint32(ip)
	if err != nil {
		return
	}

	publicIPMetric.Set(float64(ipNum))
	publicIPLastUpdate.Set(float64(time.Now().Unix()))
}

func getPublicIP() (string, error) {
	log.Println("Getting public IP")

	req, err := http.NewRequest("GET", "https://icanhazip.com", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "curl/7.79.1")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error getting public IP")
		return "", err
	}
	defer resp.Body.Close()

	ipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(ipBytes))
	if net.ParseIP(ip) == nil {
		return "", errors.New("invalid IP returned from service")
	}

	log.Println("Sucessfully retrieved public IP")

	return ip, nil
}

func convertIPv4ToUint32(ipStr string) (uint32, error) {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0, &net.AddrError{Err: "Invalid IPv4 address", Addr: ipStr}
	}
	return binary.BigEndian.Uint32(ip), nil
}
