package annotationimageset

import "errors"

const (
	Annotation		string = "img.jixingxing.io/imageset"
	KubezManager    string = "pixiu-controller-manager"
	KubezMain       string = "main"
)

var (
	// Init SafeSet which contains the HPA Average Utilization / Value
	kset = NewSafeSet(Annotation)
)

// To ensure whether we need to maintain the HPA
func IsNeedForIS(annotations map[string]string) bool {
	if annotations == nil || len(annotations) == 0 {
		return false
	}

	for aKey := range annotations {
		if kset.Has(aKey) {
			return true
		}
	}
	return false
}

func CheckAnnotation(annotations map[string]string) error {
	_, exists := annotations[Annotation]
	if !exists {
		return errors.New("Extract from annotations failed")
	}
	return nil
}