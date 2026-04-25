package registry

import "strings"

const CompanyCensusKey = "company_census"

type Dataset struct {
	Key         string
	SourceName  string
	Title       string
	ID          string
	Domain      string
	ResourceURL string
	LandingURL  string
}

var datasets = map[string]Dataset{
	CompanyCensusKey: {
		Key:         CompanyCensusKey,
		SourceName:  "dot_datahub_socrata",
		Title:       "Company Census File",
		ID:          "az4n-8mr2",
		Domain:      "data.transportation.gov",
		ResourceURL: "https://data.transportation.gov/resource/az4n-8mr2.json",
		LandingURL:  "https://data.transportation.gov/d/az4n-8mr2",
	},
}

func DatasetByKey(key string) (Dataset, bool) {
	dataset, ok := datasets[strings.ToLower(strings.TrimSpace(key))]
	return dataset, ok
}

func MustDatasetByKey(key string) Dataset {
	dataset, ok := DatasetByKey(key)
	if !ok {
		panic("unknown dataset key: " + key)
	}
	return dataset
}

func Datasets() []Dataset {
	out := make([]Dataset, 0, len(datasets))
	for _, dataset := range datasets {
		out = append(out, dataset)
	}
	return out
}
