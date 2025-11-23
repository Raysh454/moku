package model


// NewSnapshotFromResponse converts a model.Response to a model.Snapshot.
func NewSnapshotFromResponse(resp *Response) *Snapshot {
    if resp == nil {
		return nil
    }

    snap := &Snapshot{
        // ID left empty; tracker will assign one when persisting
        StatusCode: resp.StatusCode,
        URL:        resp.Request.URL,
        Body:       resp.Body, // caller may reuse resp.Body; if you want a copy, copy bytes here
        Headers:    resp.Headers,
        CreatedAt:  resp.FetchedAt,
    }

    return snap
}
