package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	admv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// package-level logger; main package should call SetLogger to provide one.
var lg = logrus.New()

// SetLogger allows the main package to inject a configured logger.
func SetLogger(l *logrus.Logger) {
	if l != nil {
		lg = l
	}
}

// writeErr writes an AdmissionResponse with an error message.
func writeErr(err error, w io.Writer) {
	lg.WithError(err).Error("writing error response")
	json.NewEncoder(w).Encode(admv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	})
}

var (
	scheme = runtime.NewScheme()
	// Codecs is exported so other packages can reuse the same deserializer
	Codecs = serializer.NewCodecFactory(scheme)
)

// AdmitFunc is an adapter type that implements http.Handler.
// It wraps a function that takes an AdmissionReview and returns an AdmissionResponse.
type AdmitFunc func(admv1.AdmissionReview) (*admv1.AdmissionResponse, error)

// ServeHTTP implements http.Handler for AdmitFunc
func (a AdmitFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeErr(fmt.Errorf("no body"), w)
		return
	}
	defer r.Body.Close()

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(err, w)
		return
	}

	var ar admv1.AdmissionReview
	_, _, err = Codecs.UniversalDeserializer().Decode(bs, nil, &ar)
	if err != nil {
		writeErr(err, w)
		return
	}

	res, err := a(ar)
	if err != nil {
		writeErr(err, w)
		return
	}

	if ar.Request != nil {
		res.UID = ar.Request.UID
	}
	ar = admv1.AdmissionReview{
		TypeMeta: ar.TypeMeta,
		Response: res,
	}

	lg.WithField("res", ar).Info("writing response")

	err = json.NewEncoder(w).Encode(ar)
	if err != nil {
		logrus.WithError(err).Error("could not serialize admissionreview")
	}
}
