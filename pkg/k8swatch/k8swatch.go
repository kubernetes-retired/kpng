package k8swatch

type Config struct {
	ConfigPath string
	APIServers []string
}

func NewConfig() *Config {
	return &Config{
		// TODO
	}
}
