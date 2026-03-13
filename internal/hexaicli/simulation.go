package hexaicli

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type simulationContextKey struct{}

type tpsSimulationSpec struct {
	min float64
	max float64
}

const defaultSimulationText = "Hexai TPS simulation mode is emitting placeholder output so you can gauge how responsive a future model might feel on your hardware. Pipe a file into stdin to preview that exact text at the configured output speed instead."

// WithCLITPSSimulation returns a context that carries the CLI TPS simulation range.
func WithCLITPSSimulation(ctx context.Context, value string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, simulationContextKey{}, strings.TrimSpace(value))
}

func tpsSimulationFromContext(ctx context.Context) (tpsSimulationSpec, bool, error) {
	if ctx == nil {
		return tpsSimulationSpec{}, false, nil
	}
	value, ok := ctx.Value(simulationContextKey{}).(string)
	if !ok || strings.TrimSpace(value) == "" {
		return tpsSimulationSpec{}, false, nil
	}
	spec, err := parseTPSSimulation(value)
	if err != nil {
		return tpsSimulationSpec{}, true, err
	}
	return spec, true, nil
}

func parseTPSSimulation(raw string) (tpsSimulationSpec, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return tpsSimulationSpec{}, fmt.Errorf("hexai: --tps-simulation expects <tps> or <min>-<max>")
	}
	if strings.Count(value, "-") == 1 && !strings.HasPrefix(value, "-") {
		return parseTPSSimulationRange(value, "-")
	}
	if strings.Count(value, ":") == 1 {
		return parseTPSSimulationRange(value, ":")
	}
	tps, err := parsePositiveTPS(value)
	if err != nil {
		return tpsSimulationSpec{}, err
	}
	return tpsSimulationSpec{min: tps, max: tps}, nil
}

func parseTPSSimulationRange(value string, sep string) (tpsSimulationSpec, error) {
	left, right, ok := strings.Cut(value, sep)
	if !ok {
		return tpsSimulationSpec{}, fmt.Errorf("hexai: invalid --tps-simulation value %q", value)
	}
	minTPS, err := parsePositiveTPS(left)
	if err != nil {
		return tpsSimulationSpec{}, err
	}
	maxTPS, err := parsePositiveTPS(right)
	if err != nil {
		return tpsSimulationSpec{}, err
	}
	if minTPS > maxTPS {
		return tpsSimulationSpec{}, fmt.Errorf("hexai: --tps-simulation minimum %.2f exceeds maximum %.2f", minTPS, maxTPS)
	}
	return tpsSimulationSpec{min: minTPS, max: maxTPS}, nil
}

func parsePositiveTPS(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	tps, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("hexai: invalid --tps-simulation value %q", value)
	}
	if tps <= 0 {
		return 0, fmt.Errorf("hexai: --tps-simulation requires a positive value, got %q", value)
	}
	return tps, nil
}

func readSimulationInput(stdin io.Reader, args []string) (string, error) {
	input, err := readInput(stdin, args)
	if err == nil {
		return input, nil
	}
	if strings.Contains(err.Error(), "no input provided") {
		return defaultSimulationText, nil
	}
	return "", err
}

func runTPSSimulation(ctx context.Context, spec tpsSimulationSpec, input string, out io.Writer) error {
	chunks := splitSimulationChunks(input)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := io.WriteString(out, chunk); err != nil {
			return err
		}
		if i == len(chunks)-1 {
			continue
		}
		if err := sleepWithContext(ctx, simulationDelay(spec, chunk, rng)); err != nil {
			return err
		}
	}
	return nil
}

func simulationDelay(spec tpsSimulationSpec, chunk string, rng *rand.Rand) time.Duration {
	tokens := estimateSimulationTokens(chunk)
	if tokens == 0 {
		return 0
	}
	tps := spec.min
	if spec.max > spec.min {
		tps += rng.Float64() * (spec.max - spec.min)
	}
	seconds := float64(tokens) / tps
	return time.Duration(seconds * float64(time.Second))
}

func splitSimulationChunks(input string) []string {
	if input == "" {
		return nil
	}
	chunks := make([]string, 0, strings.Count(input, " ")+1)
	start := 0
	sawWord := false
	for i, r := range input {
		if unicode.IsSpace(r) {
			if !sawWord {
				continue
			}
			end := advanceWhitespace(input, i)
			chunks = append(chunks, input[start:end])
			start = end
			sawWord = false
			continue
		}
		sawWord = true
	}
	if start < len(input) {
		chunks = append(chunks, input[start:])
	}
	return chunks
}

func advanceWhitespace(input string, start int) int {
	end := start
	for end < len(input) {
		r, size := utf8.DecodeRuneInString(input[end:])
		if !unicode.IsSpace(r) {
			break
		}
		end += size
	}
	return end
}

func estimateSimulationTokens(chunk string) int {
	trimmed := strings.TrimSpace(chunk)
	if trimmed == "" {
		return 0
	}
	runes := utf8.RuneCountInString(trimmed)
	return max(1, int(math.Ceil(float64(runes)/4.0)))
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
