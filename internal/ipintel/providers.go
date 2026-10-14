package ipintel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

var (
	errInvalid         = errors.New("invalid IP intelligence response")
	errAccount         = errors.New("IP intelligence account error")
	errQuota           = errors.New("IP intelligence quota exhausted")
	scamalyticsAccount = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
)

func validScamalyticsAccount(a credentials) bool {
	return scamalyticsAccount.MatchString(a.username) && (a.server == "eu" || a.server == "us")
}

func (c *Client) request(ctx context.Context, ip string, a credentials) (*http.Request, error) {
	endpoint := c.endpoint
	if c.provider == "scamalytics" && a.server == "us" && endpoint == "https://api12.scamalytics.com/v3/" {
		endpoint = "https://api11.scamalytics.com/v3/"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errInvalid
	}
	q := u.Query()
	method := http.MethodGet
	var body []byte
	switch c.provider {
	case "ipqualityscore":
		q.Set("ip", ip)
		q.Set("strictness", "0")
	case "scamalytics":
		u.Path = strings.TrimRight(u.Path, "/") + "/" + a.username
		q.Set("key", a.key)
		q.Set("ip", ip)
	case "ipapi_is":
		method = http.MethodPost
		body, _ = json.Marshal(map[string]string{"q": ip, "key": a.key})
	case "abuseipdb":
		q.Set("ipAddress", ip)
		q.Set("maxAgeInDays", "30")
	default:
		return nil, errInvalid
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, errInvalid
	}
	req.Header.Set("Accept", "application/json")
	switch c.provider {
	case "ipqualityscore":
		req.Header.Set("IPQS-KEY", a.key)
	case "ipapi_is":
		req.Header.Set("Content-Type", "application/json")
	case "abuseipdb":
		req.Header.Set("Key", a.key)
	}
	return req, nil
}

func score(value *int) *int {
	if value == nil || *value < 0 || *value > 100 {
		return nil
	}
	v := *value
	return &v
}

func fraud(result *model.IPIntelligence, value *int) {
	if result.FraudScore = score(value); result.FraudScore != nil {
		trust := 100 - *result.FraudScore
		result.TrustScore = &trust
	}
}

func matchesIP(actual, expected string) bool { ip, ok := PublicIP(actual); return ok && ip == expected }

func (c *Client) decode(body []byte, ip string) (model.IPIntelligence, error) {
	switch c.provider {
	case "ipqualityscore":
		return decodeIPQS(body)
	case "scamalytics":
		return decodeScamalytics(body, ip)
	case "ipapi_is":
		return decodeIPAPI(body, ip)
	case "abuseipdb":
		return decodeAbuseIPDB(body, ip)
	}
	return model.IPIntelligence{}, errInvalid
}

func decodeIPQS(body []byte) (model.IPIntelligence, error) {
	var d struct {
		Success        bool   `json:"success"`
		FraudScore     *int   `json:"fraud_score"`
		ConnectionType string `json:"connection_type"`
		CountryCode    string `json:"country_code"`
		Region         string `json:"region"`
		City           string `json:"city"`
		ASN            int    `json:"ASN"`
		ISP            string `json:"ISP"`
		Organization   string `json:"organization"`
		VPN            *bool  `json:"vpn"`
		Proxy          *bool  `json:"proxy"`
		Tor            *bool  `json:"tor"`
		RecentAbuse    *bool  `json:"recent_abuse"`
	}
	if json.Unmarshal(body, &d) != nil {
		return model.IPIntelligence{}, errInvalid
	}
	if !d.Success {
		return model.IPIntelligence{}, errAccount
	}
	r := model.IPIntelligence{ConnectionType: clean(d.ConnectionType), CountryCode: clean(d.CountryCode), Region: clean(d.Region), City: clean(d.City), ASN: d.ASN, ISP: clean(d.ISP), Organization: clean(d.Organization), VPN: d.VPN, Proxy: d.Proxy, Tor: d.Tor, RecentAbuse: d.RecentAbuse}
	switch r.ConnectionType {
	case "Residential", "Corporate", "Education", "Mobile", "Data Center":
		value := r.ConnectionType == "Data Center"
		r.Datacenter = &value
	}
	fraud(&r, d.FraudScore)
	return r, nil
}

func decodeScamalytics(body []byte, ip string) (model.IPIntelligence, error) {
	var d struct {
		Scamalytics struct {
			Status string `json:"status"`
			Mode   string `json:"mode"`
			IP     string `json:"ip"`
			Score  *int   `json:"scamalytics_score"`
			ISP    string `json:"scamalytics_isp"`
			Org    string `json:"scamalytics_org"`
			Proxy  struct {
				Datacenter *bool `json:"is_datacenter"`
				VPN        *bool `json:"is_vpn"`
			} `json:"scamalytics_proxy"`
			Credits struct {
				Remaining *int `json:"remaining"`
			} `json:"credits"`
		} `json:"scamalytics"`
		External struct {
			Geo struct {
				Country string `json:"ip_country_code"`
				Region  string `json:"ip_state_name"`
				City    string `json:"ip_city"`
				ASN     string `json:"asn"`
				Org     string `json:"as_name"`
			} `json:"maxmind_geolite2"`
			Lite struct {
				ProxyType string `json:"proxy_type"`
				ISP       string `json:"isp_name"`
			} `json:"ip2proxy_lite"`
			X4B struct {
				Tor *bool `json:"is_tor"`
			} `json:"x4bnet"`
		} `json:"external_datasources"`
	}
	if json.Unmarshal(body, &d) != nil {
		return model.IPIntelligence{}, errInvalid
	}
	s := d.Scamalytics
	if s.Status != "ok" {
		if s.Credits.Remaining != nil && *s.Credits.Remaining == 0 {
			return model.IPIntelligence{}, errQuota
		}
		return model.IPIntelligence{}, errAccount
	}
	if s.Mode != "live" || !matchesIP(s.IP, ip) {
		return model.IPIntelligence{}, errInvalid
	}
	r := model.IPIntelligence{VPN: s.Proxy.VPN, Datacenter: s.Proxy.Datacenter, Tor: d.External.X4B.Tor, CountryCode: clean(d.External.Geo.Country), Region: clean(d.External.Geo.Region), City: clean(d.External.Geo.City), ISP: clean(s.ISP), Organization: clean(s.Org)}
	if r.ISP == "" {
		r.ISP = clean(d.External.Lite.ISP)
	}
	if r.Organization == "" {
		r.Organization = clean(d.External.Geo.Org)
	}
	r.ASN, _ = strconv.Atoi(d.External.Geo.ASN)
	if r.Datacenter != nil && *r.Datacenter {
		r.ConnectionType = "Data Center"
	}
	switch d.External.Lite.ProxyType {
	case "PUB":
		v := true
		r.Proxy = &v
	case "0":
		v := false
		r.Proxy = &v
	}
	fraud(&r, s.Score)
	return r, nil
}

func decodeIPAPI(body []byte, ip string) (model.IPIntelligence, error) {
	var d struct {
		IP         string `json:"ip"`
		Error      string `json:"error"`
		ErrorCode  string `json:"error_code"`
		Datacenter *bool  `json:"is_datacenter"`
		Mobile     *bool  `json:"is_mobile"`
		VPN        *bool  `json:"is_vpn"`
		Proxy      *bool  `json:"is_proxy"`
		Tor        *bool  `json:"is_tor"`
		Abuser     *bool  `json:"is_abuser"`
		Company    struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Abuse string `json:"abuser_score"`
		} `json:"company"`
		ASN struct {
			ASN   int    `json:"asn"`
			Org   string `json:"org"`
			Abuse string `json:"abuser_score"`
		} `json:"asn"`
		Location struct {
			Country string `json:"country_code"`
			Region  string `json:"state"`
			City    string `json:"city"`
		} `json:"location"`
	}
	if json.Unmarshal(body, &d) != nil {
		return model.IPIntelligence{}, errInvalid
	}
	if d.Error != "" {
		if d.ErrorCode == "ERR_QUOTA_EXCEEDED" || d.ErrorCode == "ERR_FREE_TIER_EXHAUSTED" {
			return model.IPIntelligence{}, errQuota
		}
		return model.IPIntelligence{}, errAccount
	}
	if !matchesIP(d.IP, ip) {
		return model.IPIntelligence{}, errInvalid
	}
	r := model.IPIntelligence{Datacenter: d.Datacenter, VPN: d.VPN, Proxy: d.Proxy, Tor: d.Tor, Abuser: d.Abuser, ISP: clean(d.Company.Name), Organization: clean(d.ASN.Org), ASN: d.ASN.ASN, CountryCode: clean(d.Location.Country), Region: clean(d.Location.Region), City: clean(d.Location.City), CompanyAbuseScore: clean(d.Company.Abuse), ASNAbuseScore: clean(d.ASN.Abuse)}
	if d.Datacenter != nil && *d.Datacenter {
		r.ConnectionType = "Data Center"
	} else if d.Mobile != nil && *d.Mobile {
		r.ConnectionType = "Mobile"
	} else {
		r.ConnectionType = map[string]string{"hosting": "Hosting", "isp": "ISP", "business": "Corporate", "education": "Education", "government": "Government"}[d.Company.Type]
	}
	return r, nil
}

func decodeAbuseIPDB(body []byte, ip string) (model.IPIntelligence, error) {
	var d struct {
		Errors []json.RawMessage `json:"errors"`
		Data   struct {
			IP         string `json:"ipAddress"`
			Public     *bool  `json:"isPublic"`
			Confidence *int   `json:"abuseConfidenceScore"`
			Country    string `json:"countryCode"`
			Usage      string `json:"usageType"`
			ISP        string `json:"isp"`
			Tor        *bool  `json:"isTor"`
			Reports    *int   `json:"totalReports"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &d) != nil {
		return model.IPIntelligence{}, errInvalid
	}
	if len(d.Errors) > 0 {
		return model.IPIntelligence{}, errAccount
	}
	if !matchesIP(d.Data.IP, ip) || d.Data.Public != nil && !*d.Data.Public {
		return model.IPIntelligence{}, errInvalid
	}
	r := model.IPIntelligence{CountryCode: clean(d.Data.Country), ISP: clean(d.Data.ISP), Tor: d.Data.Tor, AbuseConfidenceScore: score(d.Data.Confidence), TotalReports: d.Data.Reports, ReportsWindowDays: 30}
	if r.TotalReports != nil && *r.TotalReports < 0 {
		r.TotalReports = nil
	}
	r.ConnectionType = map[string]string{"Data Center/Web Hosting/Transit": "Data Center", "Mobile ISP": "Mobile", "Fixed Line ISP": "ISP", "University/College/School": "Education", "Commercial": "Corporate", "Organization": "Corporate", "Government": "Government"}[d.Data.Usage]
	if r.ConnectionType != "" {
		v := r.ConnectionType == "Data Center"
		r.Datacenter = &v
	}
	return r, nil
}
