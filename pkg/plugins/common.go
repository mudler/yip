package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/log"
	"github.com/pkg/errors"
	"github.com/zcalusic/sysinfo"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

var system sysinfo.SysInfo

func init() {
	system.GetSysInfo()
}

type Console interface {
	Run(string, ...func(*exec.Cmd)) (string, error)
	Start(*exec.Cmd, ...func(*exec.Cmd)) error
}

// renderHelm renders the template string with helm
func renderHelm(template string, values, d map[string]interface{}) (string, error) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "",
			Version: "",
		},
		Templates: []*chart.File{
			{Name: "templates", Data: []byte(template)},
		},
		Values: map[string]interface{}{"Values": values},
	}

	v, err := chartutil.CoalesceValues(c, map[string]interface{}{"Values": d})
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}
	out, err := engine.Render(c, v)
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}

	return out["templates"], nil
}

func templateSysData(s string) string {
	interpolateOpts := map[string]interface{}{}

	data, err := json.Marshal(&system)
	if err != nil {
		log.Warning(fmt.Sprintf("Failed marshalling '%s': %s", s, err.Error()))
		return s
	}
	log.Debug(string(data))

	err = json.Unmarshal(data, &interpolateOpts)
	if err != nil {
		log.Warning(fmt.Sprintf("Failed marshalling '%s': %s", s, err.Error()))
		return s
	}
	rendered, err := renderHelm(s, map[string]interface{}{}, interpolateOpts)
	if err != nil {
		log.Warning(fmt.Sprintf("Failed rendering '%s': %s", s, err.Error()))
		return s
	}
	return rendered
}

func download(url string) (string, error) {
	var resp *http.Response
	var err error
	for i := 0; i < 10; i++ {
		resp, err = http.Get(url)
		if err == nil || strings.Contains(err.Error(), "unsupported protocol scheme") {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return "", errors.Wrap(err, "failed while getting remote pubkey")
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode/100 > 2 {
		return "", fmt.Errorf("%s %s", resp.Proto, resp.Status)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}

func isUrl(s string) bool {
	url, err := url.Parse(s)
	if err != nil || url.Scheme == "" {
		return false
	}
	return true
}
