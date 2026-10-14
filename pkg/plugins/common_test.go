package plugins

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestTemplateSysData(t *testing.T) {
	log := logrus.New()

	// Test normal template rendering
	res := templateSysData(log, "hello {{ .Values.Network }}")
	assert.NotEmpty(t, res)

	// Test missing key rendering behavior
	resMissing := templateSysData(log, "test {{ .Values.NonExistent }}")
	assert.Equal(t, "test <no value>", resMissing)
}

func TestTemplateSysDataPanicRecovery(t *testing.T) {
	log := logrus.New()

	// Test deterministic panic recovery when json marshalling panics
	origMarshal := jsonMarshal
	defer func() { jsonMarshal = origMarshal }()
	jsonMarshal = func(v any) ([]byte, error) {
		panic("simulated marshal panic")
	}

	input := "test {{ .Values.Network }}"
	res := templateSysData(log, input)
	assert.Equal(t, input, res)

	// Test recovery with nil logger
	resNil := templateSysData(nil, input)
	assert.Equal(t, input, resNil)
}
