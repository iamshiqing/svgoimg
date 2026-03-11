package svg

type ParseMode uint8

const (
	ParseIgnore ParseMode = iota
	ParseWarn
	ParseStrict
)

type Options struct {
	Mode           ParseMode
	CurveTolerance float64
}

func (o Options) withDefaults() Options {
	if o.CurveTolerance <= 0 {
		o.CurveTolerance = 0.6
	}
	return o
}
