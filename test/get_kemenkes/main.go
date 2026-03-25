// Program test untuk melihat respons raw dari GET endpoint Kemenkes.
// Berguna untuk debug kenapa Tab Referensi dan Fasyankes kosong.
//
// Cara pakai (dari root proyek):
//   go run test/get_kemenkes/main.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
)

func main() {
	// Load .env
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("[WARN] Gagal baca .env: %v\n", err)
	}

	apiURL := viper.GetString("API_URL")
	rsID   := viper.GetString("API_RS_ID")
	pass   := viper.GetString("API_PASS")

	if apiURL == "" {
		fmt.Println("[ERROR] API_URL tidak ditemukan di .env")
		os.Exit(1)
	}

	client := resty.New().
		SetTimeout(15 * time.Second).
		SetTransport(&http.Transport{DisableKeepAlives: true})

	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())

	endpoints := []struct {
		label string
		url   string
	}{
		{"GET /Referensi/tempat_tidur", apiURL + "/Referensi/tempat_tidur"},
		{"GET /Fasyankes",              apiURL + "/Fasyankes"},
	}

	for _, ep := range endpoints {
		fmt.Println(strings.Repeat("─", 65))
		fmt.Printf("  %s\n", ep.label)
		fmt.Printf("  URL : %s\n", ep.url)
		fmt.Println(strings.Repeat("─", 65))

		resp, err := client.R().
			SetHeader("X-rs-id",     rsID).
			SetHeader("X-pass",      pass).
			SetHeader("X-Timestamp", timestamp).
			Get(ep.url)

		if err != nil {
			fmt.Printf("  [ERROR] %v\n\n", err)
			continue
		}

		body := resp.Body()
		var data map[string]interface{}
		json.Unmarshal(body, &data)
		
		fmt.Printf("  Status  : %d\n", resp.StatusCode())
		for k, v := range data {
			fmt.Printf("  Key: %s\n", k)
			if list, ok := v.([]interface{}); ok && len(list) > 0 {
				itemJSON, _ := json.MarshalIndent(list[0], "    ", "    ")
				fmt.Printf("  Example Item: %s\n", string(itemJSON))
			}
		}
		fmt.Printf("\n")
	}
}
