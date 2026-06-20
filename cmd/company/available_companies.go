package company

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal"
	"github.com/Janindu-Pathirana/hex/internal/httpclient"
)

const urlCompanySymbolList = "https://www.cse.lk/chat/company_symbol_list.csv"

func availableCompaniesCacheFile() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache directory: %w", err)
	}
	return filepath.Join(cacheDir, internal.CacheDir, internal.AvailableCompaniesCache.CacheFile), nil
}

func loadCachedAvailableCompanies() ([]string, error) {
	path, err := availableCompaniesCacheFile()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat available companies cache: %w", err)
	}

	if time.Since(info.ModTime()) > internal.AvailableCompaniesCache.CacheTTL {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read available companies cache: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	companies := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		companies = append(companies, line)
	}

	if len(companies) == 0 {
		return nil, nil
	}

	return companies, nil
}

func saveAvailableCompanies(companies []string) error {
	path, err := availableCompaniesCacheFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	content := strings.Join(companies, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write available companies cache: %w", err)
	}
	return nil
}

func FetchAllAvailableCompanies() (companies []string, err error) {
	companies, err = loadCachedAvailableCompanies()
	if err != nil {
		return nil, err
	}
	if companies != nil {
		return companies, nil
	}

	resp, err := httpclient.Default.Get(urlCompanySymbolList)
	if err != nil {
		return nil, fmt.Errorf("request company symbol list: %w", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request company symbol list: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read company symbol list: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("read company symbol list: empty response")
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("parse company symbol list: missing data rows")
	}

	companies = make([]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		companies = append(companies, line)
	}

	if len(companies) == 0 {
		return nil, fmt.Errorf("parse company symbol list: no companies found")
	}

	if err := saveAvailableCompanies(companies); err != nil {
		return nil, fmt.Errorf("save available companies: %w", err)
	}

	return companies, nil
}

func FetchAvailableCompanyNames() ([]string, error) {
	companies, err := FetchAllAvailableCompanies()
	if err != nil {
		return nil, err
	}

	//VALLIBEL POWER ERATHNA PLC,VPEL.N0000

	companyNames := make([]string, 0, len(companies))
	for _, company := range companies {
		companyNames = append(companyNames, strings.Split(company, ",")[0])
	}
	return companyNames, nil
}

func FetchAvailableCompanySymbols() ([]string, error) {
	companies, err := FetchAllAvailableCompanies()
	if err != nil {
		return nil, err
	}

	//VALLIBEL POWER ERATHNA PLC,VPEL.N0000

	seen := make(map[string]bool)
	symbols := make([]string, 0, len(companies))

	for _, company := range companies {

		parts := strings.Split(company, ",")
		if len(parts) < 2 {
			continue
		}

		code := parts[1]
		symbol := strings.Split(code, ".")[0]
		symbol = strings.TrimSpace(symbol)

		if !seen[symbol] {
			seen[symbol] = true
			symbols = append(symbols, symbol)
		}
	}

	return symbols, nil
}
