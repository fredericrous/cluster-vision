package versions

import (
	"math"
	"strings"
)

// cvss3BaseScore computes the CVSS v3.0/v3.1 base score of a vector string
// such as "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" (9.8), following the
// specification's formula. ok is false when the vector is not CVSS v3 or is
// missing a base metric: a guessed score is worse than an unknown one.
func cvss3BaseScore(vector string) (score float64, ok bool) {
	if !strings.HasPrefix(vector, "CVSS:3.") {
		return 0, false
	}
	m := make(map[string]string)
	for _, part := range strings.Split(vector, "/")[1:] {
		if k, v, found := strings.Cut(part, ":"); found {
			m[k] = v
		}
	}

	weight := func(metric string, weights map[string]float64) (float64, bool) {
		w, found := weights[m[metric]]
		return w, found
	}
	scopeChanged := m["S"] == "C"
	if m["S"] != "U" && !scopeChanged {
		return 0, false
	}
	prWeights := map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	if scopeChanged {
		prWeights = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}
	}
	impactWeights := map[string]float64{"H": 0.56, "L": 0.22, "N": 0}

	av, ok1 := weight("AV", map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2})
	ac, ok2 := weight("AC", map[string]float64{"L": 0.77, "H": 0.44})
	pr, ok3 := weight("PR", prWeights)
	ui, ok4 := weight("UI", map[string]float64{"N": 0.85, "R": 0.62})
	c, ok5 := weight("C", impactWeights)
	i, ok6 := weight("I", impactWeights)
	a, ok7 := weight("A", impactWeights)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
		return 0, false
	}

	iss := 1 - (1-c)*(1-i)*(1-a)
	var impact float64
	if scopeChanged {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		impact = 6.42 * iss
	}
	if impact <= 0 {
		return 0, true
	}
	exploitability := 8.22 * av * ac * pr * ui
	if scopeChanged {
		return cvssRoundUp(math.Min(1.08*(impact+exploitability), 10)), true
	}
	return cvssRoundUp(math.Min(impact+exploitability, 10)), true
}

// cvssRoundUp is the CVSS v3.1 Roundup function: the smallest number with
// one decimal place that is >= the input, computed in integers so that
// floating-point noise (4.000000001) does not round up a whole step.
func cvssRoundUp(x float64) float64 {
	n := int64(math.Round(x * 100000))
	if n%10000 == 0 {
		return float64(n) / 100000
	}
	return float64(n/10000+1) / 10
}

// cvssSeverity maps a base score to its qualitative rating.
func cvssSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0:
		return "LOW"
	}
	return "NONE"
}
