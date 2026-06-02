package response

import "net/http"

func ContentType(contentType string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Add("Content-Type", contentType)
	}
}

func Location(loc string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Add("Location", loc)
	}
}

func Body(body []byte) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Write(body)
	}
}

func BodyFromCallback(callback func() []byte) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Write(callback())
	}
}

func Code(code int) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(code)
	}
}

func Header(key, value string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Add(key, value)
	}
}
