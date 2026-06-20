package company

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Janindu-Pathirana/hex/internal/httpclient"
)

const urlCompanyProfilePost = "https://www.cse.lk/api/companyProfile"

func FetchCompanyProfileBySymbol(companySymbol string) (string, error) {
	data := url.Values{}
	data.Set("symbol", companySymbol)

	req, err := http.NewRequest("POST", urlCompanyProfilePost, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.cse.lk")
	req.Header.Set("Referer", "https://www.cse.lk/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome Safari")

	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return "", fmt.Errorf("request company profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request company profile: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read company profile: %w", err)
	}

	return string(body), nil
}
