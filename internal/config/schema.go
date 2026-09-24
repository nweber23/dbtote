package config

type Config struct {
	Version    int                    `mapstructure:"version"`
	Defaults   Defaults               `mapstructure:"defaults"`
	Encryption Encryption             `mapstructure:"encryption"`
	Storage    map[string]StorageSpec `mapstructure:"storage"`
	Notify     Notify                 `mapstructure:"notify"`
	Targets    map[string]Target      `mapstructure:"targets"`
	Schedules  []Schedule             `mapstructure:"schedules"`
}

type Defaults struct {
	Compress  string    `mapstructure:"compress"`
	Encrypt   bool      `mapstructure:"encrypt"`
	Storage   string    `mapstructure:"storage"`
	Retention Retention `mapstructure:"retention"`
}

type Retention struct {
	KeepLast int `mapstructure:"keep_last"`
	KeepDays int `mapstructure:"keep_days"`
}

type Encryption struct {
	Recipients   []string `mapstructure:"recipients"`
	IdentityFile string   `mapstructure:"identity_file"`
}

type StorageSpec struct {
	Type      string `mapstructure:"type"`
	Path      string `mapstructure:"path"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	Prefix    string `mapstructure:"prefix"`
	Account   string `mapstructure:"account"`
	Container string `mapstructure:"container"`
}

type Notify struct {
	Slack SlackNotify `mapstructure:"slack"`
}

type SlackNotify struct {
	WebhookURLEnv string   `mapstructure:"webhook_url_env"`
	On            []string `mapstructure:"on"`
}

type Incremental struct {
	Enabled   bool   `mapstructure:"enabled"`
	Basis     string `mapstructure:"basis"`
	BinlogDir string `mapstructure:"binlog_dir"`
}

type Target struct {
	Engine      string      `mapstructure:"engine"`
	Host        string      `mapstructure:"host"`
	Port        int         `mapstructure:"port"`
	User        string      `mapstructure:"user"`
	Database    string      `mapstructure:"database"`
	Path        string      `mapstructure:"path"`         // sqlite
	URIEnv      string      `mapstructure:"uri_env"`      // mongodb
	PasswordEnv string      `mapstructure:"password_env"` // CI/container escape hatch
	SSLMode     string      `mapstructure:"sslmode"`
	Storage     string      `mapstructure:"storage"`
	Incremental Incremental `mapstructure:"incremental"`
	Retention   Retention   `mapstructure:"retention"`
}

type Schedule struct {
	ID     string `mapstructure:"id"`
	Target string `mapstructure:"target"`
	Cron   string `mapstructure:"cron"`
	Type   string `mapstructure:"type"`
}
