package gpu

import (
	"testing"
)

func TestParseNVIDIAOutputSingleGPU(t *testing.T) {
	input := `0, GPU-06319a38-c4ef-b158-198d-af30c0d01c99, NVIDIA GeForce RTX 3050 Laptop GPU, 4096, 1373, 2592, 14, 48, 9.52, 572.83`

	gpus, err := parseNVIDIAOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}

	gpu := gpus[0]

	if gpu.Index != 0 {
		t.Errorf("expected index 0, got %d", gpu.Index)
	}

	if gpu.Name != "NVIDIA GeForce RTX 3050 Laptop GPU" {
		t.Errorf("unexpected GPU name: %s", gpu.Name)
	}

	if gpu.MemoryTotalMB != 4096 {
		t.Errorf("expected total memory 4096, got %d", gpu.MemoryTotalMB)
	}

	if gpu.MemoryUsedMB != 1373 {
		t.Errorf("expected used memory 1373, got %d", gpu.MemoryUsedMB)
	}

	if gpu.MemoryFreeMB != 2592 {
		t.Errorf("expected free memory 2592, got %d", gpu.MemoryFreeMB)
	}

	if gpu.Utilization != 14 {
		t.Errorf("expected utilization 14, got %d", gpu.Utilization)
	}

	if gpu.Temperature != 48 {
		t.Errorf("expected temperature 48, got %d", gpu.Temperature)
	}

	if gpu.PowerW != 9.52 {
		t.Errorf("expected power 9.52, got %f", gpu.PowerW)
	}

	if gpu.DriverVersion != "572.83" {
		t.Errorf("unexpected driver version: %s", gpu.DriverVersion)
	}
}

func TestParseNVIDIAOutputMultipleGPUs(t *testing.T) {
	input := `0, GPU-AAAA, NVIDIA GeForce RTX 4060 Ti, 16384, 2000, 14384, 20, 45, 80.5, 572.83
1, GPU-BBBB, NVIDIA GeForce RTX 5060 Ti, 16384, 3000, 13384, 30, 50, 100.5, 572.83`

	gpus, err := parseNVIDIAOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(gpus))
	}

	if gpus[0].Name != "NVIDIA GeForce RTX 4060 Ti" {
		t.Errorf("unexpected first GPU: %s", gpus[0].Name)
	}

	if gpus[1].Name != "NVIDIA GeForce RTX 5060 Ti" {
		t.Errorf("unexpected second GPU: %s", gpus[1].Name)
	}

	if gpus[1].Index != 1 {
		t.Errorf("expected second GPU index 1, got %d", gpus[1].Index)
	}
}

func TestParseNVIDIAOutputNAValues(t *testing.T) {
	input := `0, GPU-AAAA, NVIDIA GeForce RTX 3050, 4096, 1000, 3096, N/A, N/A, N/A, 572.83`

	gpus, err := parseNVIDIAOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}

	gpu := gpus[0]

	if gpu.Utilization != 0 {
		t.Errorf("expected utilization 0 for N/A, got %d", gpu.Utilization)
	}

	if gpu.Temperature != 0 {
		t.Errorf("expected temperature 0 for N/A, got %d", gpu.Temperature)
	}

	if gpu.PowerW != 0 {
		t.Errorf("expected power 0 for N/A, got %f", gpu.PowerW)
	}
}

func TestParseNVIDIAOutputEmpty(t *testing.T) {
	gpus, err := parseNVIDIAOutput("")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(gpus))
	}
}

func TestParseNVIDIAOutputInvalidCSV(t *testing.T) {
	input := `0, GPU-AAAA, NVIDIA GeForce RTX 3050`

	_, err := parseNVIDIAOutput(input)

	if err == nil {
		t.Fatal("expected error for invalid CSV")
	}
}

func TestParseNVIDIAOutputInvalidMemory(t *testing.T) {
	input := `0, GPU-AAAA, NVIDIA GeForce RTX 3050, invalid, 1000, 3096, 20, 45, 50.0, 572.83`

	_, err := parseNVIDIAOutput(input)

	if err == nil {
		t.Fatal("expected error for invalid memory")
	}
}

func TestParseNVIDIAOutputInvalidIndex(t *testing.T) {
	input := `GPU-X, GPU-AAAA, NVIDIA GeForce RTX 3050, 4096, 1000, 3096, 20, 45, 50.0, 572.83`

	_, err := parseNVIDIAOutput(input)

	if err == nil {
		t.Fatal("expected error for invalid GPU index")
	}
}

func TestParseNVIDIAOutputInvalidUtilization(t *testing.T) {
	input := `0, GPU-AAAA, NVIDIA GeForce RTX 3050, 4096, 1000, 3096, 150, 45, 50.0, 572.83`

	_, err := parseNVIDIAOutput(input)

	if err == nil {
		t.Fatal("expected error for utilization above 100")
	}
}

func TestParseNVIDIAOutputInvalidPower(t *testing.T) {
	input := `0, GPU-AAAA, NVIDIA GeForce RTX 3050, 4096, 1000, 3096, 20, 45, invalid, 572.83`

	_, err := parseNVIDIAOutput(input)

	if err == nil {
		t.Fatal("expected error for invalid GPU power")
	}
}
