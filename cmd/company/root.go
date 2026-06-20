package company

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"encoding/json"

	customStructs "github.com/Janindu-Pathirana/hex/internal/custom-structs"
	"github.com/Janindu-Pathirana/hex/internal/httpclient"
)

const urlCompanyProfilePost = "https://www.cse.lk/api/companyProfile"

func FetchCompanyProfileBySymbol(companySymbol string) (*customStructs.CompanyProfile, error) {
	data := url.Values{}
	data.Set("symbol", companySymbol)

	req, err := http.NewRequest("POST", urlCompanyProfilePost, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.cse.lk")
	req.Header.Set("Referer", "https://www.cse.lk/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome Safari")

	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request company profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request company profile: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read company profile: %w", err)
	}

	// 1. Instantiate an empty CompanyProfile struct
	var profile customStructs.CompanyProfile

	// 2. Unmarshal the json bytes directly into the struct pointer
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("parse company profile json: %w", err)
	}

	// 3. Return the populated struct pointer
	return &profile, nil
}
