// Package timezone exposes shared time zone definitions used across the
// project. Centralizing these prevents duplicated `time.FixedZone` literals.
package timezone

import "time"

// JST is the Asia/Tokyo offset (UTC+9) without daylight saving.
var JST = time.FixedZone("JST", 9*60*60)
