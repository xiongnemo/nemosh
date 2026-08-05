package behavior

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

func ParseCase(data []byte) (Case, error) {
	var c Case
	metadata, err := toml.Decode(string(data), &c)
	if err != nil {
		return Case{}, fmt.Errorf("decode behavior case: %w", err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		return Case{}, fmt.Errorf("unknown fields: %s", strings.Join(keys, ", "))
	}
	for _, key := range []toml.Key{{"expect", "status"}, {"expect", "stdout"}, {"expect", "stderr"}} {
		if !metadata.IsDefined(key...) {
			return Case{}, fmt.Errorf("invalid behavior case: missing %s", key.String())
		}
	}
	if problems := c.Validate(); len(problems) > 0 {
		return Case{}, fmt.Errorf("invalid behavior case: %s", strings.Join(problems, "; "))
	}
	return c, nil
}
