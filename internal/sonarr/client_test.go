package sonarr

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

// seriesOneServer routes /api/v3/series, /api/v3/episode?seriesId=1, and
// /api/v3/episodefile?seriesId=1 to the series-1 fixtures.
func seriesOneServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/series":
			w.Write(fixture(t, "series.json"))
		case "/api/v3/episode":
			w.Write(fixture(t, "episodes_series1.json"))
		case "/api/v3/episodefile":
			w.Write(fixture(t, "episodefiles_series1.json"))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestSeries_HappyPath_MapsAllFields(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v3/series" {
			t.Errorf("path = %s, want /api/v3/series", r.URL.Path)
		}
		w.Write(fixture(t, "series.json"))
	})
	c := New(srv.URL, "test-key")

	series, err := c.Series(context.Background())
	if err != nil {
		t.Fatalf("Series() error = %v", err)
	}
	if len(series) != 2 {
		t.Fatalf("len(series) = %d, want 2", len(series))
	}

	s0 := series[0]
	if s0.Title != "Severance" || s0.Year != 2022 || s0.TVDBID != 371980 || s0.TMDBID != 95396 || s0.Path != "/media/tv/Severance (2022)" {
		t.Errorf("series[0] = %+v, want mapped Severance fields", s0)
	}
}

func TestSeries_EmptyArray_NoError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[]"))
	})
	c := New(srv.URL, "k")

	series, err := c.Series(context.Background())
	if err != nil {
		t.Fatalf("Series() error = %v, want nil for empty library", err)
	}
	if len(series) != 0 {
		t.Errorf("series = %v, want empty", series)
	}
}

func TestEpisodes_JoinsMetadataToFileByEpisodeFileID(t *testing.T) {
	srv := seriesOneServer(t)
	c := New(srv.URL, "k")

	episodes, err := c.Episodes(context.Background(), 1)
	if err != nil {
		t.Fatalf("Episodes() error = %v", err)
	}
	if len(episodes) != 5 {
		t.Fatalf("len(episodes) = %d, want 5", len(episodes))
	}

	e := episodes[0]
	if e.Season != 1 || e.Episode != 1 || e.Title != "Good News About Hell" {
		t.Errorf("episodes[0] metadata = %+v", e)
	}
	if !e.HasFile || e.RelativePath != "Season 01/Severance - S01E01.mkv" || e.Bytes != 3481932841 {
		t.Errorf("episodes[0] file join = %+v, want joined path/size", e)
	}
}

func TestEpisodes_HasFileFalse_NoFileLookup(t *testing.T) {
	srv := seriesOneServer(t)
	c := New(srv.URL, "k")

	episodes, err := c.Episodes(context.Background(), 1)
	if err != nil {
		t.Fatalf("Episodes() error = %v", err)
	}

	var half *Episode
	for i := range episodes {
		if episodes[i].Episode == 2 && episodes[i].Season == 1 {
			half = &episodes[i]
		}
	}
	if half == nil {
		t.Fatal("episode S01E02 not found")
	}
	if half.HasFile {
		t.Error("HasFile = true, want false")
	}
	if half.RelativePath != "" || half.Bytes != 0 {
		t.Errorf("RelativePath/Bytes = %q/%d, want zero values when hasFile is false", half.RelativePath, half.Bytes)
	}
}

func TestEpisodes_MultiEpisodeFile_BothEpisodesMapToSameFile(t *testing.T) {
	srv := seriesOneServer(t)
	c := New(srv.URL, "k")

	episodes, err := c.Episodes(context.Background(), 1)
	if err != nil {
		t.Fatalf("Episodes() error = %v", err)
	}

	var e3, e4 *Episode
	for i := range episodes {
		if episodes[i].Season == 1 && episodes[i].Episode == 3 {
			e3 = &episodes[i]
		}
		if episodes[i].Season == 1 && episodes[i].Episode == 4 {
			e4 = &episodes[i]
		}
	}
	if e3 == nil || e4 == nil {
		t.Fatal("multi-episode-file episodes not found")
	}
	wantPath := "Season 01/Severance - S01E03-E04.mkv"
	if e3.RelativePath != wantPath || e4.RelativePath != wantPath {
		t.Errorf("RelativePath = %q / %q, want both %q", e3.RelativePath, e4.RelativePath, wantPath)
	}
	if e3.Bytes != 6000000000 || e4.Bytes != 6000000000 {
		t.Errorf("Bytes = %d / %d, want both 6000000000", e3.Bytes, e4.Bytes)
	}
}

func TestEpisodes_SpecialsSeasonZero_Retained(t *testing.T) {
	srv := seriesOneServer(t)
	c := New(srv.URL, "k")

	episodes, err := c.Episodes(context.Background(), 1)
	if err != nil {
		t.Fatalf("Episodes() error = %v", err)
	}

	var found bool
	for _, e := range episodes {
		if e.Season == 0 && e.Episode == 1 {
			found = true
			if e.Title != "Special: Behind the Scenes" || !e.HasFile {
				t.Errorf("special episode = %+v, want mapped hasFile special", e)
			}
		}
	}
	if !found {
		t.Error("season 0 special not retained")
	}
}

func TestEpisodes_NoEpisodeFiles_EmptyResultNoError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/episode", "/api/v3/episodefile":
			w.Write([]byte("[]"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	c := New(srv.URL, "k")

	episodes, err := c.Episodes(context.Background(), 2)
	if err != nil {
		t.Fatalf("Episodes() error = %v, want nil", err)
	}
	if len(episodes) != 0 {
		t.Errorf("episodes = %v, want empty", episodes)
	}
}

func TestSeries_SendsAPIKeyHeader(t *testing.T) {
	var gotKey string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		w.Write([]byte("[]"))
	})
	c := New(srv.URL, "secret-key")

	if _, err := c.Series(context.Background()); err != nil {
		t.Fatalf("Series() error = %v", err)
	}
	if gotKey != "secret-key" {
		t.Errorf("X-Api-Key header = %q, want secret-key", gotKey)
	}
}

func TestSeries_Unauthorized_ReturnsErrUnauthorized(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	c := New(srv.URL, "wrong-key")

	_, err := c.Series(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestSeries_ServerError_ReturnsErrUnavailable(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := New(srv.URL, "k")

	_, err := c.Series(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestSeries_ConnectionRefused_ReturnsErrUnavailable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	c := New("http://"+addr, "k")

	_, err = c.Series(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestSeries_Timeout_ReturnsErrUnavailableWrappingDeadlineExceeded(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("[]"))
	})
	c := New(srv.URL, "k", WithTimeout(10*time.Millisecond))

	_, err := c.Series(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want it to wrap ErrUnavailable", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestSeries_MalformedJSON_ReturnsParseErrorNotPanic(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{not valid json"))
	})
	c := New(srv.URL, "k")

	_, err := c.Series(context.Background())
	if err == nil {
		t.Fatal("Series() error = nil, want parse error")
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

	if _, err := c.Series(context.Background()); err != nil {
		t.Fatalf("Series() error = %v", err)
	}
	if gotPath != "/api/v3/series" {
		t.Errorf("path = %q, want /api/v3/series (no double slash)", gotPath)
	}
}
