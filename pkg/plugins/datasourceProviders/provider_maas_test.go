package providers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("MAAS OAuth", func() {
	It("builds a PLAINTEXT Authorization header with empty consumer secret", func() {
		cfg := maasConfig{
			ConsumerKey:    "ck",
			ConsumerSecret: "",
			TokenKey:       "tk",
			TokenSecret:    "secret",
		}
		got := oauthAuthHeader(cfg, "nonce", 1)
		Expect(got).To(Equal(`OAuth oauth_consumer_key="ck", oauth_token="tk", oauth_signature_method="PLAINTEXT", oauth_signature="%26secret", oauth_nonce="nonce", oauth_timestamp="1", oauth_version="1.0"`))
	})

	It("percent-encodes reserved characters in the token secret", func() {
		cfg := maasConfig{ConsumerKey: "c k", TokenKey: "tk", TokenSecret: "a/b"}
		got := oauthAuthHeader(cfg, "n", 2)
		Expect(got).To(ContainSubstring(`oauth_consumer_key="c%20k"`))
		Expect(got).To(ContainSubstring(`oauth_signature="%26a%2Fb"`))
	})
})

var _ = Describe("MAAS preseed parsing", func() {
	It("extracts the datasource.MAAS block", func() {
		data := []byte(`#cloud-config
datasource:
  MAAS:
    metadata_url: http://maas:5248/MAAS/metadata/
    consumer_key: ckey
    consumer_secret: ""
    token_key: tkey
    token_secret: tsecret
`)
		cfg, err := parsePreseed(data)
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.MetadataURL).To(Equal("http://maas:5248/MAAS/metadata/"))
		Expect(cfg.ConsumerKey).To(Equal("ckey"))
		Expect(cfg.TokenKey).To(Equal("tkey"))
		Expect(cfg.TokenSecret).To(Equal("tsecret"))
	})

	It("errors when the MAAS block is absent", func() {
		_, err := parsePreseed([]byte("#cloud-config\nusers: []\n"))
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("MAAS cmdline parsing", func() {
	It("extracts cloud-config-url when present", func() {
		cmdline := "BOOT_IMAGE=/x ro cloud-config-url=http://maas:5248/MAAS/metadata/latest/by-id/abc/?op=get_preseed console=ttyS0"
		url, ok := cloudConfigURLFromCmdline(cmdline)
		Expect(ok).To(BeTrue())
		Expect(url).To(Equal("http://maas:5248/MAAS/metadata/latest/by-id/abc/?op=get_preseed"))
	})

	It("returns false when absent", func() {
		_, ok := cloudConfigURLFromCmdline("BOOT_IMAGE=/x ro console=ttyS0")
		Expect(ok).To(BeFalse())
	})
})

var _ = Describe("MAAS fetch", func() {
	It("signs requests and returns the field body", func() {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			if r.URL.Path == "/2012-03-01/meta-data/instance-id" {
				w.Write([]byte("i-123"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		l := logrus.New()
		p := NewMAAS(l)
		cfg := maasConfig{MetadataURL: srv.URL, ConsumerKey: "ck", TokenKey: "tk", TokenSecret: "ts"}
		body, err := p.fetch(cfg, "meta-data/instance-id")
		Expect(err).ToNot(HaveOccurred())
		Expect(string(body)).To(Equal("i-123"))
		Expect(gotAuth).To(HavePrefix("OAuth oauth_consumer_key=\"ck\""))
	})

	It("errors on a non-200 response", func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		p := NewMAAS(logrus.New())
		cfg := maasConfig{MetadataURL: srv.URL, ConsumerKey: "ck", TokenKey: "tk"}
		_, err := p.fetch(cfg, "user-data")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("MAAS Probe and Extract", func() {
	// startFakeMAAS returns a server serving the preseed and metadata fields.
	startFakeMAAS := func() *httptest.Server {
		mux := http.NewServeMux()
		mux.HandleFunc("/2012-03-01/meta-data/instance-id", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("i-abc")) })
		mux.HandleFunc("/2012-03-01/meta-data/local-hostname", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("node01")) })
		mux.HandleFunc("/2012-03-01/meta-data/public-keys", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ssh-ed25519 AAAA test")) })
		mux.HandleFunc("/2012-03-01/user-data", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("#cloud-config\n{}")) })
		return httptest.NewServer(mux)
	}

	newProbed := func(srv *httptest.Server, outDir string) *ProviderMAAS {
		// preseed server points the provider at the metadata server (srv).
		preseed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("#cloud-config\ndatasource:\n  MAAS:\n    metadata_url: " + srv.URL + "\n    consumer_key: ck\n    consumer_secret: \"\"\n    token_key: tk\n    token_secret: ts\n"))
		}))
		DeferCleanup(func() { preseed.Close() })
		cmdline := path.Join(outDir, "cmdline")
		Expect(os.WriteFile(cmdline, []byte("ro cloud-config-url="+preseed.URL), 0644)).To(Succeed())
		p := NewMAAS(logrus.New())
		p.cmdlinePath = cmdline
		p.outputDir = outDir
		return p
	}

	It("probes true and extracts hostname, keys, and userdata", func() {
		srv := startFakeMAAS()
		defer srv.Close()
		outDir, err := os.MkdirTemp("", "maas-out-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(outDir)

		p := newProbed(srv, outDir)
		Expect(p.Probe()).To(BeTrue())

		userdata, err := p.Extract()
		Expect(err).ToNot(HaveOccurred())
		Expect(string(userdata)).To(Equal("#cloud-config\n{}"))

		hn, err := os.ReadFile(path.Join(outDir, Hostname))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(hn)).To(Equal("node01"))

		keys, err := os.ReadFile(path.Join(outDir, SSH, "authorized_keys"))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(keys)).To(ContainSubstring("ssh-ed25519"))
	})

	It("probes false when there is no cloud-config-url", func() {
		outDir, err := os.MkdirTemp("", "maas-out-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(outDir)
		cmdline := path.Join(outDir, "cmdline")
		Expect(os.WriteFile(cmdline, []byte("ro console=ttyS0"), 0644)).To(Succeed())
		p := NewMAAS(logrus.New())
		p.cmdlinePath = cmdline
		Expect(p.Probe()).To(BeFalse())
	})

	It("honors the WithCmdlinePath and WithOutputDir options", func() {
		dir, err := os.MkdirTemp("", "maas-opt-*")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(dir)
		cmdline := path.Join(dir, "cmdline")
		Expect(os.WriteFile(cmdline, []byte("ro console=ttyS0"), 0644)).To(Succeed())

		p := NewMAAS(logrus.New(), WithCmdlinePath(cmdline), WithOutputDir(dir))
		Expect(p.cmdlinePath).To(Equal(cmdline))
		Expect(p.outputDir).To(Equal(dir))
	})
})
