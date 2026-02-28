package templates

import (
	"testing"

	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"github.com/stretchr/testify/assert"
)

func TestAlertClassMapping(t *testing.T) {
	tests := []struct {
		level messages.Level
		want  string
	}{
		{messages.Info, "alert-info"},
		{messages.Success, "alert-success"},
		{messages.Warning, "alert-warning"},
		{messages.Error, "alert-danger"},
		{messages.Level("unknown"), "alert-info"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			assert.Equal(t, tt.want, alertClass(tt.level))
		})
	}
}
