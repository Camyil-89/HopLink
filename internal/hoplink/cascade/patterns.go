package cascade

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Matcher holds routing rules compiled into fast string checks and fallback regexps.
type Matcher struct {
	ruleCount      int
	regexps        []*regexp.Regexp
	domainSuffixes []string
	labelRoots     []string
	tldDotSuffixes []string
	keywords       []string
	fullHosts      []string
}

// RuleCount reports how many rules were loaded (all rule kinds).
func (m Matcher) RuleCount() int {
	return m.ruleCount
}

// HasRules reports whether any routing rules are configured.
func (m Matcher) HasRules() bool {
	return m.ruleCount > 0
}

func compilePatterns(lines []string) (Matcher, error) {
	var m Matcher
	for i, line := range lines {
		line = normalizeRuleLine(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := m.addRule(line); err != nil {
			return Matcher{}, fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return m, nil
}

func (m *Matcher) addRule(line string) error {
	if after, ok := strings.CutPrefix(line, "domain:"); ok {
		if err := m.addDomainRule(strings.TrimSpace(after)); err != nil {
			return err
		}
		m.ruleCount++
		return nil
	}
	if after, ok := strings.CutPrefix(line, "keyword:"); ok {
		m.keywords = append(m.keywords, asciiLower(strings.TrimSpace(after)))
		m.ruleCount++
		return nil
	}
	if after, ok := strings.CutPrefix(line, "full:"); ok {
		m.fullHosts = append(m.fullHosts, asciiLower(strings.TrimSpace(after)))
		m.ruleCount++
		return nil
	}
	re, err := regexp.Compile(line)
	if err != nil {
		return err
	}
	m.regexps = append(m.regexps, re)
	m.ruleCount++
	return nil
}

func (m *Matcher) addDomainRule(domain string) error {
	if domain == "" {
		return fmt.Errorf("empty domain rule")
	}
	domain = asciiLower(domain)
	if !strings.Contains(domain, ".") {
		switch domain {
		case "localhost", "local", "test", "invalid", "example":
			m.labelRoots = append(m.labelRoots, domain)
		case "ru", "su", "cn", "moscow", "tatar":
			m.tldDotSuffixes = append(m.tldDotSuffixes, domain)
		default:
			if strings.HasPrefix(domain, "xn--") {
				m.tldDotSuffixes = append(m.tldDotSuffixes, domain)
			} else {
				m.labelRoots = append(m.labelRoots, domain)
			}
		}
		return nil
	}
	m.domainSuffixes = append(m.domainSuffixes, domain)
	return nil
}

func normalizeRuleLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ",")
	line = strings.Trim(line, `"'`)
	return line
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
	if m.ruleCount == 0 {
		return false
	}
	lower := asciiAllLower(value)
	if lower {
		for _, h := range m.fullHosts {
			if value == h {
				return true
			}
		}
	} else {
		for _, h := range m.fullHosts {
			if asciiEqualFold(value, h) {
				return true
			}
		}
	}
	for _, tld := range m.tldDotSuffixes {
		if hostHasTLDSuffixFold(value, tld, lower) {
			return true
		}
	}
	for _, domain := range m.domainSuffixes {
		if len(value) < len(domain) {
			continue
		}
		if hostHasSuffixDomainFold(value, domain, lower) {
			return true
		}
	}
	for _, label := range m.labelRoots {
		if len(value) < len(label) {
			continue
		}
		if hostHasSuffixDomainFold(value, label, lower) {
			return true
		}
	}
	if lower {
		for _, kw := range m.keywords {
			if asciiContainsLower(value, kw) {
				return true
			}
		}
	} else {
		for _, kw := range m.keywords {
			if asciiContainsFold(value, kw) {
				return true
			}
		}
	}
	for _, re := range m.regexps {
		if re.MatchString(value) {
			return true
		}
	}
	return false
}

func hostHasSuffixDomainFold(host, domain string, hostLower bool) bool {
	hl, dl := len(host), len(domain)
	if hl < dl {
		return false
	}
	if hl == dl {
		if hostLower {
			return host == domain
		}
		return asciiEqualFold(host, domain)
	}
	if hl <= dl+1 || host[hl-dl-1] != '.' {
		return false
	}
	tail := host[hl-dl:]
	if hostLower {
		return tail == domain
	}
	return asciiEqualFold(tail, domain)
}

func hostHasTLDSuffixFold(host, tld string, hostLower bool) bool {
	hl, tl := len(host), len(tld)
	if hl < tl+1 || host[hl-tl-1] != '.' {
		return false
	}
	tail := host[hl-tl:]
	if hostLower {
		return tail == tld
	}
	return asciiEqualFold(tail, tld)
}

func asciiAllLower(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			return false
		}
	}
	return true
}

func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func asciiContainsLower(s, sub string) bool {
	n, m := len(s), len(sub)
	if m == 0 {
		return true
	}
	if m > n {
		return false
	}
	for i := 0; i <= n-m; i++ {
		if s[i:i+m] == sub {
			return true
		}
	}
	return false
}

func asciiContainsFold(s, sub string) bool {
	n, m := len(s), len(sub)
	if m == 0 {
		return true
	}
	if m > n {
		return false
	}
	for i := 0; i <= n-m; i++ {
		match := true
		for j := 0; j < m; j++ {
			ca := s[i+j]
			if ca >= 'A' && ca <= 'Z' {
				ca += 'a' - 'A'
			}
			if ca != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func asciiLower(s string) string {
	needs := false
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func hostFromReqAddr(reqAddr string) string {
	n := len(reqAddr)
	if n == 0 {
		return reqAddr
	}
	if reqAddr[0] == '[' {
		for i := 1; i < n; i++ {
			if reqAddr[i] != ']' {
				continue
			}
			if i+2 <= n && reqAddr[i+1] == ':' {
				return reqAddr[1:i]
			}
			return reqAddr
		}
		return reqAddr
	}
	for i := n - 1; i > 0; i-- {
		if reqAddr[i] == ':' {
			return reqAddr[:i]
		}
	}
	return reqAddr
}
