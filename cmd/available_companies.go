package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/httpclient"
	"github.com/spf13/cobra"
)

const (
	urlCompanySymbolList       = "https://www.cse.lk/chat/company_symbol_list.csv"
	availableCompaniesCacheTTL = 24 * time.Hour
)

func availableCompaniesCacheFile() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "hex", "available_companies.txt"), nil
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

	if time.Since(info.ModTime()) > availableCompaniesCacheTTL {
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

var fetchAllAvailableCompanies = func() (companies []string, err error) {
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

var availableCompaniesCmd = &cobra.Command{
	Use:   "available-companies",
	Short: "Print the available companies in the stock market",
	RunE: func(cmd *cobra.Command, args []string) error {
		companies, err := fetchAllAvailableCompanies()
		if err != nil {
			return fmt.Errorf("fetch available companies: %w", err)
		}

		for _, company := range companies {
			fmt.Println(company)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(availableCompaniesCmd)
}
