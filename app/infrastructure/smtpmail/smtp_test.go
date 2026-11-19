package smtpmail

import "testing"

func TestConfigValidate(t *testing.T) {
	valid := Config{Host: "smtp.example.com", Port: 587, Security: StartTLS, Username: "a@example.com", Password: "x"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	cases := map[string]func(*Config){
		"no host":      func(c *Config) { c.Host = "" },
		"bad port":     func(c *Config) { c.Port = 0 },
		"bad security": func(c *Config) { c.Security = "ssl" },
		"bad user":     func(c *Config) { c.Username = "not-an-email" },
		"no password":  func(c *Config) { c.Password = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid
			mutate(&c)
			if err := c.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestPresetsHaveUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Presets {
		if seen[p.ID] {
			t.Errorf("duplicate preset %q", p.ID)
		}
		seen[p.ID] = true
	}
}
