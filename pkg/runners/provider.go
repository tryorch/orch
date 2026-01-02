package runners

import "github.com/go-viper/mapstructure/v2"

type ProviderConfig interface {
	IsConfigured() bool
	GetEnvVars() map[string]string
}

type AWSProviderConfig struct {
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
}

func (a *AWSProviderConfig) IsConfigured() bool {
	return a.Region != "" && a.AccessKeyID != "" && a.SecretAccessKey != ""
}

func (a *AWSProviderConfig) GetEnvVars() map[string]string {
	envs := make(map[string]string)
	if a.Region != "" {
		envs["AWS_REGION"] = a.Region
	}
	if a.AccessKeyID != "" {
		envs["AWS_ACCESS_KEY_ID"] = a.AccessKeyID
	}
	if a.SecretAccessKey != "" {
		envs["AWS_SECRET_ACCESS_KEY"] = a.SecretAccessKey
	}
	return envs
}

type AggregatedProviderConfig struct {
	AWS *AWSProviderConfig `mapstructure:"aws,omitempty"`
}

func (a *AggregatedProviderConfig) GetAllEnvVars() map[string]string {
	envs := make(map[string]string)
	if a.AWS != nil && a.AWS.IsConfigured() {
		for k, v := range a.AWS.GetEnvVars() {
			envs[k] = v
		}
	}

	return envs
}

func RetrieveProviderConfigForExecutionContext(config map[string]interface{}) (map[string]string, *AggregatedProviderConfig, error) {
	var pcfg AggregatedProviderConfig
	if err := mapstructure.Decode(config, &pcfg); err != nil {
		return nil, nil, err
	}

	return pcfg.GetAllEnvVars(), &pcfg, nil
}
