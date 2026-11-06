package usertrace

const (
	maxEventID       = 36
	maxEventType     = 16
	maxUsername      = 64
	maxProjectID     = 64
	maxIP            = 64
	maxRegion        = 64
	maxDeviceID      = 64
	maxDeviceType    = 32
	maxOS            = 64
	maxBrowser       = 64
	maxUserAgent     = 512
	maxReferer       = 1024
	maxRequestMethod = 16
	maxRequestPath   = 512
	maxRequestQuery  = 1024
	maxRoute         = 255
	maxRequestID     = 64
	maxTraceID       = 64
	maxLoginResult   = 16
	maxErrorMessage  = 255
)

func clampEvent(event Event) Event {
	event.EventID = truncate(event.EventID, maxEventID)
	event.EventType = truncate(event.EventType, maxEventType)
	event.Username = truncate(event.Username, maxUsername)
	event.ProjectID = truncate(event.ProjectID, maxProjectID)
	event.IP = truncate(event.IP, maxIP)
	event.RegionCountry = truncate(event.RegionCountry, maxRegion)
	event.RegionProvince = truncate(event.RegionProvince, maxRegion)
	event.RegionCity = truncate(event.RegionCity, maxRegion)
	event.DeviceID = normalizeDeviceID(truncate(event.DeviceID, maxDeviceID))
	event.DeviceType = truncate(event.DeviceType, maxDeviceType)
	event.OS = truncate(event.OS, maxOS)
	event.Browser = truncate(event.Browser, maxBrowser)
	event.UserAgent = truncate(event.UserAgent, maxUserAgent)
	event.Referer = truncate(event.Referer, maxReferer)
	event.RequestMethod = truncate(event.RequestMethod, maxRequestMethod)
	event.RequestPath = truncate(event.RequestPath, maxRequestPath)
	event.RequestQuery = truncate(event.RequestQuery, maxRequestQuery)
	event.Route = truncate(event.Route, maxRoute)
	event.RequestID = truncate(event.RequestID, maxRequestID)
	event.TraceID = truncate(event.TraceID, maxTraceID)
	event.LoginResult = truncate(event.LoginResult, maxLoginResult)
	event.ErrorMessage = truncate(event.ErrorMessage, maxErrorMessage)
	return event
}
