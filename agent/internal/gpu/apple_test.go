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
