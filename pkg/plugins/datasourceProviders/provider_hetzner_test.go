package providers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus"
)

// metadataDoc is a Hetzner metadata document as the native API serves it:
// YAML, with instance-id a bare integer and public-keys a block sequence.
const metadataDoc = `hostname: centos-2gb-nbg1-1
instance-id: 3449213
local-ipv4: ''
public-ipv4: 78.47.11.99
public-keys:
- ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1 paulc@example
- ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH2 second@example
`

// hetznerFixture serves a metadata document and user-data on the native
// /hetzner/v1/ routes, and reports which paths were requested.
type hetznerFixture struct {
	server   *httptest.Server
	requests []string
}

func newHetznerFixture(metadata, userdata string) *hetznerFixture {
	f := &hetznerFixture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/hetzner/v1/metadata", func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.Path)
		if metadata == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, metadata)
	})
	mux.HandleFunc("/hetzner/v1/userdata", func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.Path)
		if userdata == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, userdata)
	})
	// Anything on the retired EC2-compatible routes is a 404, as Hetzner
	// has served them since 2026-08-01.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	f.server = httptest.NewServer(mux)
	return f
}

var _ = Describe("Hetzner provider", func() {
	var (
		fixture *hetznerFixture
		outDir  string
		p       *ProviderHetzner
	)

	newProvider := func(metadata, userdata string) *ProviderHetzner {
		fixture = newHetznerFixture(metadata, userdata)
		DeferCleanup(fixture.server.Close)
		return NewHetzner(
			logrus.New(),
			WithHetznerBaseURL(fixture.server.URL+"/hetzner/v1"),
			WithHetznerOutputDir(outDir),
		)
	}

	BeforeEach(func() {
		outDir = GinkgoT().TempDir()
	})

	It("defaults to the native metadata API, not the retired EC2 routes", func() {
		p = NewHetzner(logrus.New())
		Expect(p.baseURL).To(Equal("http://169.254.169.254/hetzner/v1"))
		Expect(p.baseURL).ToNot(ContainSubstring("/latest/"))
		Expect(p.outputDir).To(Equal(ConfigPath))
	})

	Describe("Probe", func() {
		It("succeeds when the native metadata document has a hostname", func() {
			p = newProvider(metadataDoc, "#cloud-config\n")
			Expect(p.Probe()).To(BeTrue())
			Expect(fixture.requests).To(ConsistOf("/hetzner/v1/metadata"))
		})

		It("fails when the metadata service is not there", func() {
			p = newProvider("", "")
			Expect(p.Probe()).To(BeFalse())
		})

		It("fails when the document parses but carries no hostname", func() {
			p = newProvider("public-ipv4: 78.47.11.99\n", "")
			Expect(p.Probe()).To(BeFalse())
		})

		It("fails when the document is not YAML", func() {
			p = newProvider("\tnot: [yaml\n", "")
			Expect(p.Probe()).To(BeFalse())
		})
	})

	Describe("Extract", func() {
		It("returns the user-data from the native route", func() {
			p = newProvider(metadataDoc, "#cloud-config\nstages: {}\n")
			userdata, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(userdata)).To(Equal("#cloud-config\nstages: {}\n"))
			Expect(fixture.requests).To(ContainElement("/hetzner/v1/userdata"))
		})

		It("never touches the retired EC2-compatible routes", func() {
			p = newProvider(metadataDoc, "#cloud-config\n")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			for _, r := range fixture.requests {
				Expect(r).To(HavePrefix("/hetzner/v1/"))
			}
		})

		It("writes the hostname", func() {
			p = newProvider(metadataDoc, "")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.ReadFile(path.Join(outDir, Hostname))).To(BeEquivalentTo("centos-2gb-nbg1-1"))
		})

		// The regression this whole change is about: SSH keys must keep
		// being provisioned. public-keys is a YAML block sequence, so the
		// old json.Unmarshal of this value cannot parse it.
		It("writes every public key to authorized_keys", func() {
			p = newProvider(metadataDoc, "")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.ReadFile(path.Join(outDir, SSH, "authorized_keys"))).To(BeEquivalentTo(
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1 paulc@example\n" +
					"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH2 second@example\n"))
		})

		It("keeps authorized_keys owner-only", func() {
			p = newProvider(metadataDoc, "")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			info, err := os.Stat(path.Join(outDir, SSH, "authorized_keys"))
			Expect(err).ToNot(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		It("writes no authorized_keys when the server has no keys", func() {
			p = newProvider("hostname: nokeys\n", "")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			_, err = os.Stat(path.Join(outDir, SSH, "authorized_keys"))
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("writes the public IPv4 and the instance ID", func() {
			p = newProvider(metadataDoc, "")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.ReadFile(path.Join(outDir, "public_ipv4"))).To(BeEquivalentTo("78.47.11.99"))
			// instance-id is a bare integer in the document.
			Expect(os.ReadFile(path.Join(outDir, "instance_id"))).To(BeEquivalentTo("3449213"))
		})

		It("writes local_ipv4 only when the server is on a private network", func() {
			p = newProvider(metadataDoc, "")
			_, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			// local-ipv4 is '' in the fixture.
			_, err = os.Stat(path.Join(outDir, "local_ipv4"))
			Expect(os.IsNotExist(err)).To(BeTrue())

			outDir = GinkgoT().TempDir()
			p = newProvider("hostname: h\nlocal-ipv4: 10.0.0.3\n", "")
			_, err = p.Extract()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.ReadFile(path.Join(outDir, "local_ipv4"))).To(BeEquivalentTo("10.0.0.3"))
		})

		It("still provisions hostname and keys when user-data is absent", func() {
			p = newProvider(metadataDoc, "")
			userdata, err := p.Extract()
			Expect(err).ToNot(HaveOccurred())
			Expect(userdata).To(BeNil())
			Expect(os.ReadFile(path.Join(outDir, Hostname))).To(BeEquivalentTo("centos-2gb-nbg1-1"))
			Expect(os.ReadFile(path.Join(outDir, SSH, "authorized_keys"))).ToNot(BeEmpty())
		})

		It("fails when the metadata document is unreachable", func() {
			p = newProvider("", "#cloud-config\n")
			_, err := p.Extract()
			Expect(err).To(HaveOccurred())
		})

		It("fails when the document carries no hostname", func() {
			p = newProvider("public-ipv4: 78.47.11.99\n", "")
			_, err := p.Extract()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("hostname"))
		})
	})
})
