package proxies

import (
	"strconv"
	"strings"
	"sync"

	"github.com/biter777/countries"
)

var (
	counter     = make(map[string]int)
	counterLock = sync.Mutex{}
)

type RenameParts struct {
	CountryFlag string
	Country     string
	Index       int
	ShortID     string
	Name        string
}

func Rename(name string) string {
	return NextRenameParts(name).Name
}

func NextRenameParts(name string) RenameParts {
	counterLock.Lock()
	defer counterLock.Unlock()

	country := strings.ToUpper(strings.TrimSpace(name))
	counterKey := country
	if counterKey == "" {
		counterKey = "Other"
	}
	flag := countryFlag(country)
	if flag == "❓" {
		country = "Other"
	}

	counter[counterKey]++
	index := counter[counterKey]
	shortIDCountry := country
	if shortIDCountry == "Other" {
		shortIDCountry = "OT"
	}
	shortID := shortIDCountry + strconv.Itoa(index)
	legacyName := CountryCodeToFlag(name) + strings.TrimSpace(name) + "_" + strconv.Itoa(index)
	if strings.TrimSpace(name) == "" {
		legacyName = CountryCodeToFlag(name) + "_" + strconv.Itoa(index)
	}

	return RenameParts{
		CountryFlag: flag,
		Country:     country,
		Index:       index,
		ShortID:     shortID,
		Name:        legacyName,
	}
}

// ResetRenameCounter resets all counters to 0.
func ResetRenameCounter() {
	counterLock.Lock()
	defer counterLock.Unlock()

	counter = make(map[string]int)
}

func CountryCodeToFlag(countryCode string) string {
	flag := countryFlag(countryCode)
	if flag == "❓" {
		return "❓Other"
	}
	return flag
}

func countryFlag(countryCode string) string {
	code := strings.ToUpper(countryCode)
	country := countries.ByName(code)
	if country == countries.Unknown {
		return "❓"
	}
	return country.Emoji()
}
