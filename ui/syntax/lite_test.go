package syntax

import "testing"

func TestLiteKillSwitchFieldHighlight(t *testing.T) {
	for _, value := range []string{"on", "off", "true", "false", "1", "0"} {
		text := "[Interface]\nBlockUntunneledTraffic = " + value + "\n"
		spans := highlightConfig(text)
		for _, span := range spans {
			if span.t == highlightError {
				t.Fatalf("valid value %q marked invalid", value)
			}
		}
	}
}
