package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

type CloudflareUpdate struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Content string `json:"content"`
	Ttl string `json:"ttl"`
	Proxied bool `json:"proxied"`
}

type Cloudflare struct {
	Success  bool                `json:"success"`
	Errors   []string            `json:"errors"`
	Messages []string            `json:"messages"`
	Result   []map[string]string `json:"result"`
}

type Configuration struct {
	Email         string `yaml:"email"`
	Method        string `yaml:"method"`
	Key           string `yaml:"key"`
	ZoneId        string `yaml:"zone_id"`
	RecordName    string `yaml:"record_name"`
	Ttl           string `yaml:"time_to_live"`
	Proxy         bool   `yaml:"proxy"`
	Notifications map[string]map[string]string
}

type IpAddress struct {
	RawIp string `json:"ip"`
}

func (a *IpAddress) Parse() net.IP {
	return net.ParseIP(a.RawIp)
}

func (a *IpAddress) IsIPv4() bool {
	ip := a.Parse()
	return ip != nil && ip.To4() != nil
}

func (a *IpAddress) IsIPv6() bool {
	ip := a.Parse()
	return ip != nil && ip.To4() == nil
}

func (a *IpAddress) GetRecordType() string {
	if a.IsIPv4() {
		return "A"
	} else if a.IsIPv6() {
		return "AAAA"
	} else {
		return ""
	}
}

func main() {
	if len(os.Args[1:]) < 1 {
		log.Fatalf("expect at least 1 arg")
	}
	f := os.Args[1]
	yml, err := ioutil.ReadFile(f)
	if err != nil {
		log.Fatalf("unable to load configuration file due to %v", err)
	}

	config := make(map[string]Configuration)
	err = yaml.Unmarshal(yml, &config)
	if err != nil {
		log.Fatalf("unable to unmarshal configuration due to %v", err)
		return
	}

	url := "https://api64.ipify.org?format=json"
	response, err := http.Get(url)
	if err != nil {
		log.Fatalf("network failure due to %v", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalf("failed to close network resources due to %v", err)
		}
	}(response.Body)

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("failed to read response due to %v", err)
	}

	var public IpAddress
	err = json.Unmarshal(b, &public)
	if err != nil {
		log.Fatalf("failed to unmarshal data due to %v", err)
	}

	for k, v := range config {
		if k == "cloudflare" {
			email := v.Email
			method := v.Method
			key := v.Key
			zoneId := v.ZoneId
			recordName := v.RecordName
			ttl := v.Ttl
			proxy := v.Proxy
			//notifications := v.Notifications
			// https://api.cloudflare.com/#dns-records-for-a-zone-list-dns-records
			cfUrl := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s", zoneId, public.GetRecordType(), recordName)
			verify, err := http.NewRequest("GET", cfUrl, nil)
			if err != nil {
				log.Fatalf("unable to construct request for cloudflare due to %v", err)
			}
			verify.Header.Add("X-Auth-Email", email)
			if method == "global" {
				verify.Header.Add("X-Auth-Key", key)
			} else {
				verify.Header.Add("Authorization", "Bearer "+key)
			}
			verify.Header.Add("Content-Type", "application/json")
			client := &http.Client{}
			v, err := client.Do(verify)
			if err != nil {
				log.Fatalf("unable to send http request to cloudflare api due to %v", err)
			}

			vv, err := ioutil.ReadAll(v.Body)
			if err != nil {
				log.Fatalf("failed to read response due to %v", err)
			}
			var record Cloudflare
			err = json.Unmarshal(vv, &record)
			if err != nil {
				log.Fatalf("unable to unmarshal external data due to %v", err)
			}

			if !record.Success {
				log.Fatalf("request to cloudflare api was not successful due to %v", record.Errors)
			}

			if len(record.Result) == 0 {
				log.Fatalf("record not found, it needs to be created (ip: %s, zone: %s)", public.RawIp, zoneId)
			}

			for _, result := range record.Result {
				if result["type"] == public.GetRecordType() {
					if result["content"] == public.RawIp {
						log.Printf("no need to update record, ip has not changed (type: %s, current public ip: %s, ip on file: %s", public.GetRecordType(), public.RawIp, result["content"])
						os.Exit(0)
					} else {
						rid := result["id"]
						uu, err := json.Marshal(&CloudflareUpdate{
							Type: public.GetRecordType(),
							Name: recordName,
							Content: public.RawIp,
							Ttl: ttl,
							Proxied: proxy,
						})
						// https://api.cloudflare.com/#dns-records-for-a-zone-patch-dns-record
						updateUrl := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneId, rid)
						update, err := http.NewRequest("PATCH", updateUrl, bytes.NewBuffer(uu))
						if err != nil {
							log.Fatalf("unable to construct request for cloudflare due to %v", err)
						}
						update.Header.Add("X-Auth-Email", email)
						if method == "global" {
							update.Header.Add("X-Auth-Key", key)
						} else {
							update.Header.Add("Authorization", "Bearer "+key)
						}
						update.Header.Add("Content-Type", "application/json")
						client := &http.Client{}
						u, err := client.Do(update)
						if err != nil {
							log.Fatalf("unable to send http request to cloudflare api due to %v", err)
						}

						body, err := ioutil.ReadAll(u.Body)
						if err != nil {
							log.Fatalf("unable to read data from response due to %v", err)
						}
						var updateResult Cloudflare
						err = json.Unmarshal(body, &updateResult)
						if err != nil {
							log.Fatalf("unable to unmarshal response from request due to %v", err)
						}
						if updateResult.Success { // todo: add alerts here
							log.Printf("successfully updated dns record, %v", updateResult.Result)
						} else {
							log.Fatalf("unable to update dynamic dns record due to %v (additional messages: %v)", updateResult.Errors, updateResult.Messages)
						}
					}
				}
			}
		}
	}
}
