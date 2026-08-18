package radarr

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return data
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestMovies_HappyPath_MapsAllFields(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v3/movie" {
			t.Errorf("path = %s, want /api/v3/movie", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "movies.json"))
	})
	c := New(srv.URL, "test-key")

	movies, err := c.Movies(context.Background())
	if err != nil {
		t.Fatalf("Movies() error = %v", err)
	}
	if len(movies) != 3 {
		t.Fatalf("len(movies) = %d, want 3", len(movies))
	}

	m0 := movies[0]
	if m0.Title != "Inception" || m0.Year != 2010 || m0.TMDBID != 27205 || m0.IMDbID != "tt1375666" {
		t.Errorf("movies[0] = %+v, want mapped Inception fields", m0)
	}
	if m0.RelativePath != "Inception.mkv" || m0.Bytes != 3481932841 {
		t.Errorf("movies[0] file fields = %+v", m0)
	}
	if !m0.HasFile {
		t.Errorf("movies[0].HasFile = false, want true")
	}

	m1 := movies[1]
	if m1.HasFile {
		t.Errorf("movies[1].HasFile = true, want false (movieFile is null)")
	}
	if m1.RelativePath != "" || m1.Bytes != 0 {
		t.Errorf("movies[1] file fields = %+v, want zero values when hasFile is false", m1)
	}

	m2 := movies[2]
	if m2.TMDBID != 0 {
		t.Errorf("movies[2].TMDBID = %d, want 0 (missing tmdb match)", m2.TMDBID)
	}
	if !m2.HasFile {
		t.Errorf("movies[2].HasFile = false, want true - a missing TMDB match must not abort the fetch")
	}
}

func TestMovies_SendsAPIKeyHeader(t *testing.T) {
	var gotKey string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		w.Write([]byte("[]"))
	})
	c := New(srv.URL, "secret-key")

	if _, err := c.Movies(context.Background()); err != nil {
		t.Fatalf("Movies() error = %v", err)
	}
	if gotKey != "secret-key" {
		t.Errorf("X-Api-Key header = %q, want secret-key", gotKey)
	}
}

func TestMovies_EmptyArray_NoError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[]"))
	})
	c := New(srv.URL, "k")

	movies, err := c.Movies(context.Background())
	if err != nil {
		t.Fatalf("Movies() error = %v, want nil for empty library", err)
	}
	if len(movies) != 0 {
		t.Errorf("movies = %v, want empty", movies)
	}
}

func TestMovies_Unauthorized_ReturnsErrUnauthorized(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	c := New(srv.URL, "wrong-key")

	_, err := c.Movies(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestMovies_ServerError_ReturnsErrUnavailable(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := New(srv.URL, "k")

	_, err := c.Movies(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestMovies_ConnectionRefused_ReturnsErrUnavailable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := l.Addr().String()
	l.Close() // free the port immediately so nothing listens there

	c := New("http://"+addr, "k")

	_, err = c.Movies(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestMovies_Timeout_ReturnsErrUnavailableWrappingDeadlineExceeded(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("[]"))
	})
	c := New(srv.URL, "k", WithTimeout(10*time.Millisecond))

	_, err := c.Movies(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want it to wrap ErrUnavailable", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestMovies_MalformedJSON_ReturnsParseErrorNotPanic(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{not valid json"))
	})
	c := New(srv.URL, "k")

	_, err := c.Movies(context.Background())
	if err == nil {
		t.Fatal("Movies() error = nil, want parse error")
	}
}

func TestPing_OK_ReturnsNil(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/system/status" {
			t.Errorf("path = %s, want /api/v3/system/status", r.URL.Path)
		}
		w.Write(fixture(t, "system_status.json"))
	})
	c := New(srv.URL, "k")

	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestPing_ServiceUnavailable_ReturnsErrUnavailable(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	c := New(srv.URL, "k")

	err := c.Ping(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestNew_TrailingSlashOnBaseURL_ProducesSameRequestPath(t *testing.T) {
	var gotPath string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte("[]"))
	})
	c := New(srv.URL+"/", "k")

	if _, err := c.Movies(context.Background()); err != nil {
		t.Fatalf("Movies() error = %v", err)
	}
	if gotPath != "/api/v3/movie" {
		t.Errorf("path = %q, want /api/v3/movie (no double slash)", gotPath)
	}
	if strings.Contains(gotPath, "//") {
		t.Errorf("path = %q, contains double slash", gotPath)
	}
}
