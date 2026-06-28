package main

// MockData contains realistic demo responses for each intent ID.
// Each entry matches the intentResponse JSON shape that the /intent handler
// produces in live mode, so the frontend renders identically in both modes.
//
// NormalizedResult shapes used here:
//   transaction → { output_type, source, latency_ms, transactions: [...] }
//   activity    → { output_type, source, latency_ms, activities:   [...] }
//   generic     → { output_type, source, latency_ms, generic:      {...} }

var MockData = map[string]any{

	"recent_purchases": map[string]any{
		"intent_id":        "recent_purchases",
		"label":            "Recent Purchases",
		"partial_failure":  false,
		"total_latency_ms": int64(84),
		"results": []any{
			map[string]any{
				"output_type": "transaction",
				"source":      "amazon.order_history",
				"latency_ms":  int64(84),
				"transactions": []any{
					map[string]any{
						"id": "113-4829301-7392847", "merchant": "Amazon",
						"amount": 2499.00, "currency": "INR",
						"date": "2026-06-25", "category": "Electronics",
						"raw_source": "amazon.order_history",
					},
					map[string]any{
						"id": "408-2910384-1029384", "merchant": "Amazon",
						"amount": 899.00, "currency": "INR",
						"date": "2026-06-18", "category": "Books",
						"raw_source": "amazon.order_history",
					},
					map[string]any{
						"id": "405-7293847-2039485", "merchant": "Amazon",
						"amount": 3299.00, "currency": "INR",
						"date": "2026-06-10", "category": "Home & Kitchen",
						"raw_source": "amazon.order_history",
					},
				},
			},
		},
	},

	"professional_activity": map[string]any{
		"intent_id":        "professional_activity",
		"label":            "Professional Activity",
		"partial_failure":  false,
		"total_latency_ms": int64(230),
		"results": []any{
			map[string]any{
				"output_type": "activity",
				"source":      "linkedin.activity_feed",
				"latency_ms":  int64(112),
				"activities": []any{
					map[string]any{
						"platform": "linkedin", "type": "post",
						"timestamp":  "2026-06-26",
						"summary":    "Published: 'Building a unified data layer over Wire API in Go — WireOS'",
						"url":        "https://linkedin.com/posts/tharun",
						"raw_source": "linkedin.activity_feed",
					},
					map[string]any{
						"platform": "linkedin", "type": "connection",
						"timestamp":  "2026-06-24",
						"summary":    "Connected with Priya Nair — Platform Engineer at Last9",
						"url":        "",
						"raw_source": "linkedin.activity_feed",
					},
					map[string]any{
						"platform": "linkedin", "type": "comment",
						"timestamp":  "2026-06-21",
						"summary":    "Commented on 'Go concurrency patterns for high-throughput systems'",
						"url":        "",
						"raw_source": "linkedin.activity_feed",
					},
				},
			},
			map[string]any{
				"output_type": "generic",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(118),
				"generic": map[string]any{
					"headline":    "B.Tech Mining + BS Data Science | Go Backend & Platform Engineering",
					"location":    "Surathkal, Karnataka, India",
					"connections": 412,
					"profile_url": "https://linkedin.com/in/tharun",
				},
			},
		},
	},

	"github_activity": map[string]any{
		"intent_id":        "github_activity",
		"label":            "GitHub Activity",
		"partial_failure":  false,
		"total_latency_ms": int64(201),
		"results": []any{
			map[string]any{
				"output_type": "activity",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(189),
				"activities": []any{
					map[string]any{
						"platform": "github", "type": "push",
						"timestamp":  "2026-06-27",
						"summary":    "Pushed 4 commits to Tharun-bot/wireos — feat: executor fan-out, normalizer types, demo data",
						"url":        "https://github.com/Tharun-bot/wireos",
						"raw_source": "anakin.agentic_search",
					},
					map[string]any{
						"platform": "github", "type": "push",
						"timestamp":  "2026-06-25",
						"summary":    "Pushed 2 commits to Tharun-bot/promptshield — fix: garak adapter nil check on empty probe list",
						"url":        "https://github.com/Tharun-bot/promptshield",
						"raw_source": "anakin.agentic_search",
					},
				},
			},
			map[string]any{
				"output_type": "activity",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(201),
				"activities": []any{
					map[string]any{
						"platform": "github", "type": "pull_request",
						"timestamp":  "2026-06-26",
						"summary":    "PR #14 open — feat: add /catalog endpoint with Wire API integration",
						"url":        "https://github.com/Tharun-bot/wireos/pull/14",
						"raw_source": "anakin.agentic_search",
					},
					map[string]any{
						"platform": "github", "type": "pull_request",
						"timestamp":  "2026-06-20",
						"summary":    "PR #12 merged — fix: resolveParams skips empty user_input gracefully",
						"url":        "https://github.com/Tharun-bot/wireos/pull/12",
						"raw_source": "anakin.agentic_search",
					},
				},
			},
		},
	},

	"portfolio_snapshot": map[string]any{
		"intent_id":        "portfolio_snapshot",
		"label":            "Portfolio Snapshot",
		"partial_failure":  false,
		"total_latency_ms": int64(167),
		"results": []any{
			map[string]any{
				"output_type": "generic",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(143),
				"generic": map[string]any{
					"total_value":    142580.50,
					"currency":       "INR",
					"day_change":     1240.30,
					"day_change_pct": 0.88,
					"as_of":          "2026-06-27",
				},
			},
			map[string]any{
				"output_type": "generic",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(167),
				"generic": map[string]any{
					"positions": []any{
						map[string]any{"ticker": "RELIANCE", "name": "Reliance Industries", "qty": 5, "avg_cost": 2710.00, "ltp": 2884.60, "pnl": 873.00, "exchange": "NSE"},
						map[string]any{"ticker": "INFY", "name": "Infosys Ltd", "qty": 10, "avg_cost": 1480.00, "ltp": 1523.45, "pnl": 434.50, "exchange": "NSE"},
						map[string]any{"ticker": "GOLDBEES", "name": "Nippon India Gold ETF", "qty": 50, "avg_cost": 54.20, "ltp": 57.85, "pnl": 182.50, "exchange": "BSE"},
					},
				},
			},
		},
	},

	"job_applications": map[string]any{
		"intent_id":        "job_applications",
		"label":            "Job Applications",
		"partial_failure":  false,
		"total_latency_ms": int64(156),
		"results": []any{
			map[string]any{
				"output_type": "activity",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(156),
				"activities": []any{
					map[string]any{
						"platform": "linkedin", "type": "application",
						"timestamp":  "2026-06-20",
						"summary":    "Applied: Go Backend Engineer @ Last9 — status: under review",
						"url":        "",
						"raw_source": "anakin.agentic_search",
					},
					map[string]any{
						"platform": "linkedin", "type": "application",
						"timestamp":  "2026-06-15",
						"summary":    "Applied: Platform Engineer @ SigNoz — status: recruiter screen scheduled",
						"url":        "",
						"raw_source": "anakin.agentic_search",
					},
					map[string]any{
						"platform": "linkedin", "type": "application",
						"timestamp":  "2026-06-08",
						"summary":    "Applied: SWE Intern @ WarpBuild — status: no response",
						"url":        "",
						"raw_source": "anakin.agentic_search",
					},
				},
			},
			map[string]any{
				"output_type": "generic",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(134),
				"generic": map[string]any{
					"saved_jobs": []any{
						map[string]any{"title": "Go Engineer", "company": "Temporal.io", "location": "Remote-worldwide"},
						map[string]any{"title": "Platform Infra Engineer", "company": "Zenskar", "location": "Bengaluru"},
						map[string]any{"title": "Backend Engineer", "company": "Turso", "location": "Remote-worldwide"},
					},
				},
			},
		},
	},

	"web_research": map[string]any{
		"intent_id":        "web_research",
		"label":            "Web Research",
		"partial_failure":  false,
		"total_latency_ms": int64(310),
		"results": []any{
			map[string]any{
				"output_type": "generic",
				"source":      "anakin.agentic_search",
				"latency_ms":  int64(310),
				"generic": map[string]any{
					"query":   "Wire API personal data federation",
					"summary": "Wire (by Anakin) is a personal data API that provides authenticated, structured access to your digital accounts — Amazon, LinkedIn, GitHub, Robinhood and more — through a single unified REST interface. Responses are normalised across platforms by the Wire runtime.",
					"sources": []any{
						map[string]any{"title": "Wire API Docs", "url": "https://anakin.io/wire"},
						map[string]any{"title": "Agentic Search", "url": "https://anakin.io/docs/agentic-search"},
					},
				},
			},
		},
	},
}

// GetMockResponse returns the demo payload for a given intent ID.
// The returned map matches the intentResponse JSON shape exactly.
func GetMockResponse(intentID string) map[string]any {
	if data, ok := MockData[intentID]; ok {
		return data.(map[string]any)
	}
	return map[string]any{
		"intent_id":        intentID,
		"label":            intentID,
		"partial_failure":  false,
		"total_latency_ms": int64(0),
		"results":          []any{},
	}
}
