// Package update tells argoxd whether a newer release exists on GitHub.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseURL = "https://api.github.com/repos/awcodify/argoxd/releases/latest"
	checkInterval    = 24 * time.Hour
	requestTimeout   = 3 * time.Second
	maxResponseBytes = 1 << 20
)

var releaseNumber = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// parse splits a plain release number such as "0.4.1" or "v0.4.1". Builds
// that carry a pre-release or build suffix are not releases and parse as false.
func parse(version string) ([3]int, bool) {
	var numbers [3]int
	match := releaseNumber.FindStringSubmatch(strings.TrimSpace(version))
	if match == nil {
		return numbers, false
	}
	for index := range numbers {
		number, err := strconv.Atoi(match[index+1])
		if err != nil {
			return numbers, false
		}
		numbers[index] = number
	}
	return numbers, true
}

// Newer reports whether latest is a later release than current. It is false
// when either is not a plain release number, such as a development build.
func Newer(current, latest string) bool {
	have, ok := parse(current)
	if !ok {
		return false
	}
	want, ok := parse(latest)
	if !ok {
		return false
	}
	for index := range have {
		if want[index] != have[index] {
			return want[index] > have[index]
		}
	}
	return false
}

// Checker looks up the latest release, asking GitHub at most once per Interval.
type Checker struct {
	URL       string
	CachePath string
	Interval  time.Duration
	Client    *http.Client
	Now       func() time.Time
}

// NewChecker returns a Checker for argoxd's GitHub releases that caches in the
// user's cache directory.
func NewChecker() Checker {
	checker := Checker{URL: latestReleaseURL, Interval: checkInterval}
	if dir, err := os.UserCacheDir(); err == nil {
		checker.CachePath = filepath.Join(dir, "argoxd", "update.json")
	}
	return checker
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checkedAt"`
	Latest    string    `json:"latest"`
}

func (c Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c Checker) readCache() (cacheEntry, bool) {
	var entry cacheEntry
	if c.CachePath == "" {
		return entry, false
	}
	data, err := os.ReadFile(c.CachePath)
	if err != nil || json.Unmarshal(data, &entry) != nil || entry.Latest == "" {
		return entry, false
	}
	return entry, true
}

func (c Checker) writeCache(latest string) {
	if c.CachePath == "" {
		return
	}
	data, err := json.Marshal(cacheEntry{CheckedAt: c.now(), Latest: latest})
	if err != nil || os.MkdirAll(filepath.Dir(c.CachePath), 0o700) != nil {
		return
	}
	_ = os.WriteFile(c.CachePath, data, 0o600)
}

// Latest returns the newest release number without a leading "v". It reuses
// the saved answer until it is Interval old. A failed request is an error and
// leaves the saved answer alone.
func (c Checker) Latest(ctx context.Context) (string, error) {
	if entry, ok := c.readCache(); ok && c.now().Sub(entry.CheckedAt) < c.Interval {
		return entry.Latest, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "argoxd")
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release: %s", response.Status)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&release); err != nil {
		return "", err
	}
	latest := strings.TrimPrefix(release.Tag, "v")
	if latest == "" {
		return "", errors.New("latest release has no tag")
	}
	c.writeCache(latest)
	return latest, nil
}

// Available returns the newer release to announce, or "" when current is up to
// date, is not a release, or the check fails. It never reports an error: an
// update notice must not get in the way, for example behind a proxy.
func (c Checker) Available(ctx context.Context, current string) string {
	if _, ok := parse(current); !ok {
		return ""
	}
	latest, err := c.Latest(ctx)
	if err != nil || !Newer(current, latest) {
		return ""
	}
	return latest
}
