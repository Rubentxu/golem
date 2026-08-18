package profile

// DevProfileDefaults is the canonical dev profile — all in-memory adapters.
// It is the baseline: every test that does not explicitly set a profile
// must behave identically to dev.
var DevProfileDefaults = Profile{
	Version: 1,
	Name:    "dev",
	Adapters: map[string]string{
		"journal":    "memstore",
		"graph":      "memstore",
		"registry":   "memstore",
		"transport":  "memstore",
		"checkpoint": "memstore",
		"search":     "memstore",
	},
	Options: nil,
}

// DurableProfileDefaults is the canonical durable profile.
// It selects bbolt for the journal and natsjs for transport.
// Actual file-based values (path, URL) come from profiles/durable.yaml.
var DurableProfileDefaults = Profile{
	Version: 1,
	Name:    "durable",
	Adapters: map[string]string{
		"journal":    "bbolt",
		"graph":      "memstore",
		"registry":   "memstore",
		"transport":  "natsjs",
		"checkpoint": "memstore",
		"search":     "memstore",
	},
	Options: map[string]any{
		"bbolt": map[string]any{
			"path": "./var/golem.journal",
		},
	},
}
