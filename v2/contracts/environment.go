package contracts

type Environment interface {
	LookupEnv(key string) (value string, set bool)
}
