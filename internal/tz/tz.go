// Package tz は本プロジェクト共通のタイムゾーン定義を提供する。
package tz

import "time"

// JST は日本標準時 (UTC+9)。
var JST = time.FixedZone("JST", 9*60*60)
