# Cloudflare Dynamic DNS Updater (Go)

written by Darren Rambaud

## Why?

A simple CLI app to update dynamic DNS settings for your CloudFlare account. Useful if you have a home lab and a non-static 
IP address. Avoids the need to manually update your vanity domain name if your ISP decides to change your public IP. Very 
similar to [K0p1-Git/cloudflare-ddns-updater](https://github.com/K0p1-Git/cloudflare-ddns-updater) except it is written 
in everybody's favorite programming language, Go.

### Features
1. externalized configuration to yml file
2. support for records of multiple types (A for IPv4, and AAAA for IPv6) 

## How?

### pre-requisites
1. Go 1.17+
2. Active Cloudflare account

```
$ git clone git@github.com:xyzst/cloudflare-dynamic-dns-updater-go.git
$ cd cloudflare-dynamic-dns-updater-go
$ go build ./...
$ ./cloudflare-ddns-updater-cli /full/path/to/configuration.yml
```

## Improvements

[ ] refactor for readability
[ ] add alert integration with popular messaging services (ie, slack)
