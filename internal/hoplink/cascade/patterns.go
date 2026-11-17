package cascade

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

// Matcher holds compiled regular expressions for host or SNI routing.
type Matcher struct {
	patterns []*regexp.Regexp
}

func compilePatterns(lines []string) (Matcher, error) {
	var m Matcher
	for i, line := range lines {
		line = normalizeRuleLine(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re, err := compileLine(line)
		if err != nil {
			return Matcher{}, fmt.Errorf("line %d: %w", i+1, err)
		}
		m.patterns = append(m.patterns, re)
	}
	return m, nil
}

func normalizeRuleLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ",")
	line = strings.Trim(line, `"'`)
	return line
}

// compileLine accepts Clash/sing-box style rules or raw regex:
//   - domain:host — host and subdomains (TLD labels like ru → *.ru)
//   - keyword:text — substring match
//   - full:host — exact hostname
//   - otherwise treated as Go regexp
func compileLine(line string) (*regexp.Regexp, error) {
	if after, ok := strings.CutPrefix(line, "domain:"); ok {
		return domainRuleRegexp(after)
	}
	if after, ok := strings.CutPrefix(line, "keyword:"); ok {
		return regexp.Compile("(?i)" + regexp.QuoteMeta(strings.TrimSpace(after)))
	}
	if after, ok := strings.CutPrefix(line, "full:"); ok {
		host := strings.TrimSpace(after)
		return regexp.Compile("(?i)^" + regexp.QuoteMeta(host) + "$")
	}
	return regexp.Compile(line)
}

func domainRuleRegexp(domain string) (*regexp.Regexp, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("empty domain rule")
	}
	esc := regexp.QuoteMeta(domain)
	if !strings.Contains(domain, ".") {
		switch domain {
		case "localhost", "local", "test", "invalid", "example":
			return regexp.Compile(`(?i)^(?:.+\.)?` + esc + `$`)
		case "ru", "su", "cn", "moscow", "tatar":
			return regexp.Compile(`(?i)\.` + esc + `$`)
		default:
			if strings.HasPrefix(domain, "xn--") {
				return regexp.Compile(`(?i)\.` + esc + `$`)
			}
			return regexp.Compile(`(?i)^(?:.+\.)?` + esc + `$`)
		}
	}
	return regexp.Compile(`(?i)^(?:.+\.)?` + esc + `$`)
}

// LoadPatternsFile reads one regex per line from path. An empty path yields an empty matcher.
func LoadPatternsFile(path string) (Matcher, error) {
	if strings.TrimSpace(path) == "" {
		return Matcher{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return Matcher{}, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return Matcher{}, err
	}
	return compilePatterns(lines)
}

// LoadPatternsFromText compiles patterns from a multi-line string (used in tests).
func LoadPatternsFromText(text string) (Matcher, error) {
	return compilePatterns(strings.Split(text, "\n"))
}

func (m Matcher) Match(value string) bool {
	if len(m.patterns) == 0 {
		return false
	}
	for _, re := range m.patterns {
		if re.MatchString(value) {
			return true
		}
	}
	return false
}

func hostFromReqAddr(reqAddr string) string {
	host, _, err := net.SplitHostPort(reqAddr)
	if err != nil {
		return reqAddr
	}
	return host
}