package wifi

import (
	"context"
	"os/exec"
)

func read(ctx context.Context) (reading, error) {
	out, err := exec.CommandContext(ctx, "system_profiler", "SPAirPortDataType", "-json").Output()
	if err != nil {
		return reading{}, err
	}
	return parseDarwin(out)
}
