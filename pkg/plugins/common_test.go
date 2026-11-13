package plugins

import (
	"testing"

	"github.com/mudler/yip/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestTemplateSysDataPanicRecovery(t *testing.T) {
	log := logger.NewDummy()

	// Test normal/safe template execution
	res := templateSysData(log, "hello {{ .Values.Network }}")
	assert.NotNil(t, res)

	// Test that panic recovery mechanism functions properly
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("templateSysData leaked a panic: %v", r)
		}
	}()

	res2 := templateSysData(log, "test {{ .Values.NonExistent }}")
	assert.Equal(t, "test {{ .Values.NonExistent }}", res2)
}
