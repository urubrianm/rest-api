package services

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

const (
	exportDomainFlag            = "export-domain"
	exportPremiumDomainFlag     = "export-premium-domain"
	exportUseSubdomainsFlag     = "export-use-subdomains"
	exportSubdomainsK8SPoolFlag = "export-subdomains-k8s-pool"
	exportApiKeyFlag            = "export-api-key"
	exportApiSecretFlag         = "export-api-secret"
	exportApiRoleFlag           = "export-api-role"
	exportPathPrefixFlag        = "export-path-prefix"

	// torrent-http-proxy signing for external playback URLs
	exportProxyApiKeyFlag    = "export-proxy-api-key"
	exportProxyApiSecretFlag = "export-proxy-api-secret"
	exportProxyTokenTtlFlag  = "export-proxy-token-ttl"
)

const (
	videoInfoServiceHostFlag = "video-info-host"
	videoInfoServicePortFlag = "video-info-port"
)

func RegisterVideoInfoServiceFlags(f []cli.Flag) []cli.Flag {
	return append(f,
		cli.StringFlag{
			Name:   videoInfoServiceHostFlag,
			Usage:  "video info service host",
			EnvVar: "VIDEO_INFO_SERVICE_HOST",
			Value:  "",
		},
		cli.IntFlag{
			Name:   videoInfoServicePortFlag,
			Usage:  "video info service port",
			EnvVar: "VIDEO_INFO_SERVICE_PORT",
			Value:  0,
		},
	)
}

func RegisterExportFlags(f []cli.Flag) []cli.Flag {
	return append(f,
		cli.StringFlag{
			Name:   exportDomainFlag,
			Usage:  "export domain",
			Value:  "",
			EnvVar: "EXPORT_DOMAIN",
		},
		cli.StringFlag{
			Name:   exportPremiumDomainFlag,
			Usage:  "export premium domain",
			Value:  "",
			EnvVar: "EXPORT_PREMIUM_DOMAIN",
		},
		cli.StringFlag{
			Name:   exportApiKeyFlag,
			Usage:  "export api key",
			Value:  "",
			EnvVar: "EXPORT_API_KEY",
		},
		cli.StringFlag{
			Name:   exportApiSecretFlag,
			Usage:  "export api token",
			Value:  "",
			EnvVar: "EXPORT_API_SECRET",
		},
		cli.StringFlag{
			Name:   exportApiRoleFlag,
			Usage:  "export api role",
			Value:  "free",
			EnvVar: "EXPORT_API_ROLE",
		},
		cli.BoolTFlag{
			Name:   exportUseSubdomainsFlag,
			Usage:  "export use subdomains",
			EnvVar: "EXPORT_USE_SUBDOMAINS",
		},
		cli.StringFlag{
			Name:   exportSubdomainsK8SPoolFlag,
			Usage:  "export k8s pool",
			EnvVar: "EXPORT_K8S_POOL",
			Value:  "seeder",
		},
		cli.StringFlag{
			Name:   exportPathPrefixFlag,
			Usage:  "export path prefix",
			EnvVar: "EXPORT_PATH_PREFIX",
			Value:  "/",
		},
		cli.StringFlag{
			Name:   exportProxyApiKeyFlag,
			Usage:  "torrent-http-proxy api key (used to sign export URLs for external players)",
			EnvVar: "EXPORT_PROXY_API_KEY",
			Value:  "",
		},
		cli.StringFlag{
			Name:   exportProxyApiSecretFlag,
			Usage:  "torrent-http-proxy api secret (used to sign export URLs for external players)",
			EnvVar: "EXPORT_PROXY_API_SECRET",
			Value:  "",
		},
		cli.IntFlag{
			Name:   exportProxyTokenTtlFlag,
			Usage:  "torrent-http-proxy token TTL in seconds (default: 600)",
			EnvVar: "EXPORT_PROXY_TOKEN_TTL",
			Value:  600,
		},
	)
}

type ExportType string

const (
	ExportTypeDownload    ExportType = "download"
	ExportTypeStream      ExportType = "stream"
	ExportTypeTorrentStat ExportType = "torrent_client_stat"
	ExportTypeSubtitles   ExportType = "subtitles"
	ExportTypeMediaProbe  ExportType = "media_probe"
	// ExportTypeAndroid is a convenience export that points to a REST endpoint returning
	// a ready-to-play HLS URL for Android players.
	ExportTypeAndroid ExportType = "android_player"
)

var ExportTypes = []ExportType{
	ExportTypeDownload,
	ExportTypeStream,
	ExportTypeTorrentStat,
	ExportTypeSubtitles,
	ExportTypeMediaProbe,
	ExportTypeAndroid,
}

type ExportGetArgs struct {
	Types []ExportType
}

type Export struct {
	exporters []Exporter
}

func ExportGetArgsFromParams(g ParamGetter) (*ExportGetArgs, error) {
	var types []ExportType
	if g.Query("types") != "" {
		for _, k := range strings.Split(g.Query("types"), ",") {
			kk := strings.TrimSpace(k)
			found := false
			for _, t := range ExportTypes {
				if string(t) == kk {
					types = append(types, t)
					found = true
					break
				}
			}
			if !found {
				return nil, errors.Errorf("failed to parse export type \"%v\"", kk)
			}
		}
	} else {
		types = ExportTypes
	}
	return &ExportGetArgs{
		Types: types,
	}, nil
}

type Exporter interface {
	Type() ExportType
	Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error)
}

func NewExport(e ...Exporter) *Export {
	return &Export{
		exporters: e,
	}
}

func (s *Export) Get(r *Resource, i *ListItem, args *ExportGetArgs, g ParamGetter) (*ExportResponse, error) {
	items := map[string]ExportItem{}
	for _, t := range args.Types {
		for _, e := range s.exporters {
			if e.Type() == t {
				ex, err := e.Export(r, i, g)
				if err != nil {
					return nil, err
				}
				if ex != nil {
					items[ex.Type] = *ex
				}
			}
		}
	}
	return &ExportResponse{
		Source:      *i,
		ExportItems: items,
	}, nil
}

type BaseExporter struct {
	ub         *URLBuilder
	exportType ExportType
}

func (s *BaseExporter) Type() ExportType {
	return s.exportType
}

func (s *BaseExporter) BuildURL(r *Resource, i *ListItem, g ParamGetter) (*MyURL, error) {
	return s.ub.Build(r, i, g, s.Type())
}

type DownloadExporter struct {
	BaseExporter
}

type StreamExporter struct {
	tb *TagBuilder
	BaseExporter
}

type MediaProbeExporter struct {
	BaseExporter
}

type TorrentStatExporter struct {
	BaseExporter
}

type SubtitlesExporter struct {
	BaseExporter
}

// AndroidPlayerExporter returns a URL to the REST endpoint that generates a
// player-friendly HLS URL (and optional subtitles) for Android clients.
//
// Why this exists:
// - The regular `stream` export is only returned for media items.
// - For directory items (like a season folder), clients still need a stable
//   endpoint to request a playable URL once they pick a file.
type AndroidPlayerExporter struct {
	ub *URLBuilder
}

func NewDownloadExporter(ub *URLBuilder) *DownloadExporter {
	return &DownloadExporter{
		BaseExporter: BaseExporter{
			ub:         ub,
			exportType: ExportTypeDownload,
		},
	}
}

func (s *DownloadExporter) Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error) {
	url, err := s.BuildURL(r, i, g)
	if err != nil {
		return nil, err
	}

	return &ExportItem{
		Type: string(s.Type()),
		URL:  url.String(),
		ExportMetaItem: ExportMetaItem{
			Meta: url.BuildExportMeta(),
		},
	}, nil
}

func NewStreamExporter(ub *URLBuilder, tb *TagBuilder) *StreamExporter {
	return &StreamExporter{
		BaseExporter: BaseExporter{
			ub:         ub,
			exportType: ExportTypeStream,
		},
		tb: tb,
	}
}

func (s *StreamExporter) Type() ExportType {
	return ExportTypeStream
}

func (s *StreamExporter) MakeExportStreamItem(r *Resource, i *ListItem, g ParamGetter) (*ExportStreamItem, error) {
	ei := &ExportStreamItem{}
	t, err := s.tb.Build(r, i, g)
	if err != nil {
		return nil, err
	}
	if t != nil {
		ei.Tag = t
	}
	return ei, nil
}

func (s *StreamExporter) Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error) {
	if i.MediaFormat == "" {
		return nil, nil
	}
	url, err := s.BuildURL(r, i, g)
	if err != nil {
		return nil, err
	}

	ei, err := s.MakeExportStreamItem(r, i, g)
	if err != nil {
		return nil, err
	}

	return &ExportItem{
		Type:             string(s.Type()),
		URL:              url.String(),
		ExportStreamItem: *ei,
		ExportMetaItem: ExportMetaItem{
			Meta: url.BuildExportMeta(),
		},
	}, nil
}

func NewTorrentStatExporter(ub *URLBuilder) *TorrentStatExporter {
	return &TorrentStatExporter{
		BaseExporter: BaseExporter{
			ub:         ub,
			exportType: ExportTypeTorrentStat,
		},
	}
}

func (s *TorrentStatExporter) Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error) {
	url, err := s.BuildURL(r, i, g)
	if err != nil {
		return nil, err
	}
	if url == nil {
		return nil, nil
	}

	return &ExportItem{
		Type: string(s.Type()),
		URL:  url.String(),
	}, nil
}

func NewSubtitlesExporter(c *cli.Context, ub *URLBuilder) *SubtitlesExporter {
	if c.String(videoInfoServiceHostFlag) == "" && c.Int(videoInfoServicePortFlag) == 0 {
		return nil
	}
	return &SubtitlesExporter{
		BaseExporter: BaseExporter{
			ub:         ub,
			exportType: ExportTypeSubtitles,
		},
	}
}

func NewAndroidPlayerExporter(ub *URLBuilder) *AndroidPlayerExporter {
	return &AndroidPlayerExporter{ub: ub}
}

func (s *AndroidPlayerExporter) Type() ExportType {
	return ExportTypeAndroid
}

func (s *AndroidPlayerExporter) Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error) {
	// Build absolute URL to this rest-api endpoint.
	bubc := BaseURLBuilder{
		sd:                s.ub.sd,
		cm:                s.ub.cm,
		r:                 r,
		i:                 i,
		g:                 g,
		domain:            s.ub.domain,
		premiumDomain:     s.ub.premiumDomain,
		apiKey:            s.ub.apiKey,
		apiSecret:         s.ub.apiSecret,
		apiRole:           s.ub.apiRole,
		useSubdomains:     s.ub.useSubdomains,
		subdomainsK8SPool: s.ub.subdomainsK8SPool,
		pathPrefix:        s.ub.pathPrefix,
	}

	u := &MyURL{}
	var err error
	u, err = bubc.BuildScheme(u)
	if err != nil {
		return nil, err
	}
	u, err = bubc.BuildDomain(u)
	if err != nil {
		return nil, err
	}

	// Keep it stable: always point to the android-player endpoint.
	u.Path = "/resource/" + r.ID + "/android-player"
	q := u.Query()

	// If the export is requested for a specific content item (file), include its
	// relative path so Android can call this directly without an extra lookup.
	if i != nil && i.Type != ListTypeDirectory && strings.Trim(i.PathStr, "/") != "" {
		q.Set("path", strings.Trim(i.PathStr, "/"))
	}
	u.RawQuery = q.Encode()

	return &ExportItem{
		Type: string(s.Type()),
		URL:  u.String(),
	}, nil
}

func (s *SubtitlesExporter) Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error) {
	if i.MediaFormat != Video {
		return nil, nil
	}
	url, err := s.BuildURL(r, i, g)
	if err != nil {
		return nil, err
	}
	if url == nil {
		return nil, nil
	}
	return &ExportItem{
		Type: string(s.Type()),
		URL:  url.String(),
	}, nil
}

func NewMediaProbeExporter(ub *URLBuilder) *MediaProbeExporter {
	return &MediaProbeExporter{
		BaseExporter: BaseExporter{
			ub:         ub,
			exportType: ExportTypeMediaProbe,
		},
	}
}

func (s *MediaProbeExporter) Export(r *Resource, i *ListItem, g ParamGetter) (*ExportItem, error) {
	url, err := s.BuildURL(r, i, g)
	if err != nil {
		return nil, err
	}
	if url == nil {
		return nil, nil
	}
	return &ExportItem{
		Type: string(s.Type()),
		URL:  url.String(),
	}, nil
}
