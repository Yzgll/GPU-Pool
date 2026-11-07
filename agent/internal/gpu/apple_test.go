package gpu

import "testing"

func TestParseAppleOutputSingleGPU(t *testing.T) {
	input := `{
		"SPDisplaysDataType": [
			{
				"_name": "Apple M5 Pro",
				"sppci_model": "Apple M5 Pro",
				"spdisplays_vendor": "sppci_vendor_Apple",
				"spdisplays_mtlgpufamilysupport": "spdisplays_metal4"
			}
		]
	}`

	devices, err := parseAppleOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(devices))
	}

	d := devices[0]

	if d.Name != "Apple M5 Pro" {
		t.Errorf("expected name Apple M5 Pro, got %s", d.Name)
	}

	if d.Vendor != "Apple" {
		t.Errorf("expected vendor Apple, got %s", d.Vendor)
	}

	if d.Metal != "Metal 4" {
		t.Errorf("expected Metal 4, got %s", d.Metal)
	}
}

func TestParseAppleOutputMultipleGPUs(t *testing.T) {
	input := `{
		"SPDisplaysDataType": [
			{
				"sppci_model": "Apple M5 Pro",
				"spdisplays_vendor": "sppci_vendor_Apple",
				"spdisplays_mtlgpufamilysupport": "spdisplays_metal4"
			},
			{
				"sppci_model": "AMD Radeon Pro 5500M",
				"spdisplays_vendor": "sppci_vendor_AMD",
				"spdisplays_mtlgpufamilysupport": "spdisplays_metal2"
			}
		]
	}`

	devices, err := parseAppleOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(devices))
	}

	if devices[0].Name != "Apple M5 Pro" || devices[0].Vendor != "Apple" {
		t.Errorf("unexpected first GPU: %+v", devices[0])
	}

	if devices[1].Name != "AMD Radeon Pro 5500M" || devices[1].Vendor != "AMD" {
		t.Errorf("unexpected second GPU: %+v", devices[1])
	}

	if devices[0].Metal != "Metal 4" {
		t.Errorf("unexpected first GPU metal: %s", devices[0].Metal)
	}

	if devices[1].Metal != "Metal 2" {
		t.Errorf("unexpected second GPU metal: %s", devices[1].Metal)
	}
}

func TestParseAppleOutputEmptyName(t *testing.T) {
	input := `{
		"SPDisplaysDataType": [
			{
				"sppci_model": "",
				"spdisplays_vendor": "sppci_vendor_Apple"
			}
		]
	}`

	devices, err := parseAppleOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(devices))
	}

	if devices[0].Name != "Apple GPU" {
		t.Errorf("expected fallback name Apple GPU, got %s", devices[0].Name)
	}
}

func TestParseAppleOutputInvalidJSON(t *testing.T) {
	_, err := parseAppleOutput("not json")

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNormalizeAppleMetal(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"spdisplays_metal4", "Metal 4"},
		{"spdisplays_metal3", "Metal 3"},
		{"spdisplays_metal2", "Metal 2"},
		{"metal4", "Metal 4"},
		{"", ""},
	}

	for _, c := range cases {
		got := normalizeAppleMetal(c.input)
		if got != c.want {
			t.Errorf("normalizeAppleMetal(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestNormalizeAppleEnum(t *testing.T) {
	cases := []struct {
		input    string
		fallback string
		want     string
	}{
		{"sppci_vendor_Apple", "Apple", "Apple"},
		{"sppci_vendor_AMD", "Apple", "AMD"},
		{"", "Apple", "Apple"},
		{"Apple", "Apple", "Apple"},
	}

	for _, c := range cases {
		got := normalizeAppleEnum(c.input, c.fallback)
		if got != c.want {
			t.Errorf("normalizeAppleEnum(%q, %q) = %q, want %q", c.input, c.fallback, got, c.want)
		}
	}
}

func TestParsePowermetricsOutput(t *testing.T) {
	input := `Machine model: MacBookPro18,3
OS version: 15.0.1

**** GPU usage ****
GPU HW active frequency: 1296 MHz
GPU HW active residency:  68.45% (1296 MHz:  68% 636 MHz:   0% 414 MHz:   0%)
GPU SW requested state: (1296 MHz: 100% 636 MHz:   0% 414 MHz:   0%)
GPU SW state: (1296 MHz: 100% 636 MHz:   0% 414 MHz:   0%)
GPU idle residency:  31.55%
GPU Power: 3.90 W
`

	utilization, powerW := parsePowermetricsOutput(input)

	if utilization != 68 {
		t.Errorf("expected utilization 68, got %d", utilization)
	}

	if powerW != 3.90 {
		t.Errorf("expected power 3.90, got %f", powerW)
	}
}

func TestParsePowermetricsOutputEmpty(t *testing.T) {
	utilization, powerW := parsePowermetricsOutput("")

	if utilization != 0 {
		t.Errorf("expected utilization 0, got %d", utilization)
	}

	if powerW != 0 {
		t.Errorf("expected power 0, got %f", powerW)
	}
}

func TestParsePowermetricsOutputClampsOver100(t *testing.T) {
	input := `GPU HW active residency: 123.45% (1296 MHz: 100%)
GPU Power: 3.90 W
`

	utilization, _ := parsePowermetricsOutput(input)

	if utilization != 100 {
		t.Errorf("expected utilization clamped to 100, got %d", utilization)
	}
}

func TestParsePercentAfterColon(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"GPU HW active residency:  68.45% (1296 MHz: 68%)", 68.45},
		{"GPU HW active residency:  0.00%", 0},
		{"GPU HW active residency:  100.0%", 100},
		{"no colon here", -1},
		{"no percent: 12.3", -1},
	}

	for _, c := range cases {
		got := parsePercentAfterColon(c.input)
		if got != c.want {
			t.Errorf("parsePercentAfterColon(%q) = %f, want %f", c.input, got, c.want)
		}
	}
}

func TestParseWattAfterColon(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"GPU Power: 3.90 W", 3.90},
		{"GPU Power: 0 W", 0},
		{"GPU Power: 12.5 W", 12.5},
		{"GPU Power: 28 mW", 0.028},
		{"GPU Power: 500 mW", 0.5},
		{"GPU Power: 1200 µW", 0.0012},
		{"no colon", -1},
		{"GPU Power:", -1},
	}

	for _, c := range cases {
		got := parseWattAfterColon(c.input)
		if got != c.want {
			t.Errorf("parseWattAfterColon(%q) = %f, want %f", c.input, got, c.want)
		}
	}
}

func TestParsePowermetricsOutputRealM5(t *testing.T) {
	// 基于 M5 Pro 真机 powermetrics 输出的真实片段，
	// 验证 mW 单位能被正确转换为 W，利用率被四舍五入。
	input := `Machine model: Mac17,8
OS version: 25E246

**** GPU usage ****

GPU HW active frequency: 338 MHz
GPU HW active residency:   5.02% (338 MHz: 5.0% 486 MHz:   0%)
GPU idle residency:  94.98%
GPU Power: 28 mW
`

	utilization, powerW := parsePowermetricsOutput(input)

	if utilization != 5 {
		t.Errorf("expected utilization 5, got %d", utilization)
	}

	if powerW != 0.028 {
		t.Errorf("expected power 0.028 W, got %f", powerW)
	}
}
