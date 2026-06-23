package providers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/yip/pkg/logger"
	"gopkg.in/yaml.v3"
)

type maasConfig struct {
	MetadataURL    string
	ConsumerKey    string
	ConsumerSecret string
	TokenKey       string
	TokenSecret    string
}

// MAAS authenticates metadata requests with OAuth 1.0 "PLAINTEXT" signatures.
// We only need two small pieces of that scheme (RFC 3986 percent-encoding and
// the Authorization header below), so they are implemented inline here on
// purpose instead of importing a full OAuth library such as
// github.com/dghubble/oauth1 or the MAAS-specific github.com/juju/gomaasapi.
// yip is embedded into Kairos and many other images, so we keep its dependency
// surface deliberately small and avoid pulling a transitive tree in for ~25
// lines of signing logic.

// percentEncode escapes a string per RFC 3986 unreserved-character rules,
// matching OAuth parameter encoding.
func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// oauthAuthHeader builds an OAuth 1.0 PLAINTEXT Authorization header value.
func oauthAuthHeader(cfg maasConfig, nonce string, timestamp int64) string {
	sig := percentEncode(cfg.ConsumerSecret) + "%26" + percentEncode(cfg.TokenSecret)
	return fmt.Sprintf(
		`OAuth oauth_consumer_key="%s", oauth_token="%s", oauth_signature_method="PLAINTEXT", oauth_signature="%s", oauth_nonce="%s", oauth_timestamp="%d", oauth_version="1.0"`,
		percentEncode(cfg.ConsumerKey),
		percentEncode(cfg.TokenKey),
		sig,
		percentEncode(nonce),
		timestamp,
	)
}

type preseedDoc struct {
	Datasource struct {
		MAAS struct {
			MetadataURL    string `yaml:"metadata_url"`
			ConsumerKey    string `yaml:"consumer_key"`
			ConsumerSecret string `yaml:"consumer_secret"`
			TokenKey       string `yaml:"token_key"`
			TokenSecret    string `yaml:"token_secret"`
		} `yaml:"MAAS"`
	} `yaml:"datasource"`
}

// parsePreseed extracts the MAAS datasource config from a cloud-config preseed.
func parsePreseed(data []byte) (maasConfig, error) {
	var doc preseedDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return maasConfig{}, err
	}
	m := doc.Datasource.MAAS
	if m.MetadataURL == "" || m.ConsumerKey == "" || m.TokenKey == "" {
		return maasConfig{}, errors.New("MAAS: preseed has no usable datasource.MAAS block")
	}
	return maasConfig{
		MetadataURL:    m.MetadataURL,
		ConsumerKey:    m.ConsumerKey,
		ConsumerSecret: m.ConsumerSecret,
		TokenKey:       m.TokenKey,
		TokenSecret:    m.TokenSecret,
	}, nil
}

const cloudConfigURLKey = "cloud-config-url="

// cloudConfigURLFromCmdline returns the cloud-config-url value from a kernel
// command line, if present.
func cloudConfigURLFromCmdline(cmdline string) (string, bool) {
	for _, field := range strings.Fields(cmdline) {
		if strings.HasPrefix(field, cloudConfigURLKey) {
			return strings.TrimPrefix(field, cloudConfigURLKey), true
		}
	}
	return "", false
}

const maasMetadataVersion = "2012-03-01"

// ProviderMAAS implements the Provider interface for MAAS.
type ProviderMAAS struct {
	l           logger.Interface
	client      *http.Client
	cmdlinePath string
	outputDir   string
	nonceFn     func() string
	timestampFn func() int64

	cfg   *maasConfig
	cfgOK bool
}

// Option configures a ProviderMAAS. Production code uses the defaults; tests
// and out-of-package harnesses use these to redirect the command-line source
// and output directory.
type Option func(*ProviderMAAS)

// WithCmdlinePath overrides where the provider reads the kernel command line
// from (default /proc/cmdline).
func WithCmdlinePath(p string) Option {
	return func(m *ProviderMAAS) { m.cmdlinePath = p }
}

// WithOutputDir overrides where the provider writes hostname and SSH keys
// (default ConfigPath).
func WithOutputDir(d string) Option {
	return func(m *ProviderMAAS) { m.outputDir = d }
}

// NewMAAS returns a new ProviderMAAS with production defaults, optionally
// adjusted by opts.
func NewMAAS(l logger.Interface, opts ...Option) *ProviderMAAS {
	p := &ProviderMAAS{
		l:           l,
		client:      &http.Client{Timeout: 30 * time.Second},
		cmdlinePath: "/proc/cmdline",
		outputDir:   ConfigPath,
		nonceFn:     func() string { return strconv.FormatInt(time.Now().UnixNano(), 10) },
		timestampFn: func() int64 { return time.Now().Unix() },
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *ProviderMAAS) String() string { return "MAAS" }

// fetch GETs a single metadata field, OAuth-signed.
func (p *ProviderMAAS) fetch(cfg maasConfig, field string) ([]byte, error) {
	url := strings.TrimRight(cfg.MetadataURL, "/") + "/" + maasMetadataVersion + "/" + field
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("MAAS: building request: %w", err)
	}
	req.Header.Set("Authorization", oauthAuthHeader(cfg, p.nonceFn(), p.timestampFn()))
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MAAS: contacting metadata service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAAS: status %d for %s", resp.StatusCode, field)
	}
	return io.ReadAll(resp.Body)
}

// discover reads the kernel command line, fetches the MAAS preseed, and parses
// it into a maasConfig.
func (p *ProviderMAAS) discover() (maasConfig, error) {
	cmdline, err := os.ReadFile(p.cmdlinePath)
	if err != nil {
		return maasConfig{}, fmt.Errorf("MAAS: reading cmdline: %w", err)
	}
	url, ok := cloudConfigURLFromCmdline(string(cmdline))
	if !ok {
		return maasConfig{}, errors.New("MAAS: no cloud-config-url on cmdline")
	}
	resp, err := p.client.Get(url)
	if err != nil {
		return maasConfig{}, fmt.Errorf("MAAS: fetching preseed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return maasConfig{}, fmt.Errorf("MAAS: preseed status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return maasConfig{}, fmt.Errorf("MAAS: reading preseed body: %w", err)
	}
	return parsePreseed(body)
}

// Probe returns true when a MAAS datasource is discoverable from the cmdline.
func (p *ProviderMAAS) Probe() bool {
	cfg, err := p.discover()
	if err != nil {
		p.l.Debugf("MAAS: probe failed: %s", err)
		return false
	}
	p.cfg = &cfg
	p.cfgOK = true
	return true
}

// Extract fetches MAAS metadata, writes hostname and SSH keys, and returns the
// raw user-data.
func (p *ProviderMAAS) Extract() ([]byte, error) {
	if !p.cfgOK {
		cfg, err := p.discover()
		if err != nil {
			return nil, err
		}
		p.cfg = &cfg
		p.cfgOK = true
	}
	cfg := *p.cfg

	hostname, err := p.fetch(cfg, "meta-data/local-hostname")
	if err != nil {
		return nil, fmt.Errorf("MAAS: required local-hostname: %w", err)
	}
	if _, err := p.fetch(cfg, "meta-data/instance-id"); err != nil {
		return nil, fmt.Errorf("MAAS: required instance-id: %w", err)
	}
	if err := os.WriteFile(path.Join(p.outputDir, Hostname), hostname, 0644); err != nil {
		return nil, fmt.Errorf("MAAS: writing hostname: %w", err)
	}

	if keys, err := p.fetch(cfg, "meta-data/public-keys"); err == nil && len(keys) > 0 {
		if err := os.MkdirAll(path.Join(p.outputDir, SSH), 0755); err != nil {
			return nil, fmt.Errorf("MAAS: creating ssh dir: %w", err)
		}
		if err := os.WriteFile(path.Join(p.outputDir, SSH, "authorized_keys"), keys, 0600); err != nil {
			return nil, fmt.Errorf("MAAS: writing ssh keys: %w", err)
		}
	} else if err != nil {
		p.l.Warnf("MAAS: no public-keys: %s", err)
	}

	userdata, err := p.fetch(cfg, "user-data")
	if err != nil {
		p.l.Debugf("MAAS: no user-data: %s", err)
		return nil, nil
	}
	return userdata, nil
}
