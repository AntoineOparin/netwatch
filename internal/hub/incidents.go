package hub

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/incident"
	"github.com/AntoineOparin/netwatch/internal/series"
)

const defaultIncidentHistory = time.Hour

var incidentLocations = map[incident.Location]netwatchv1.IncidentLocation{
	incident.LocationLocal:    netwatchv1.IncidentLocation_INCIDENT_LOCATION_LOCAL,
	incident.LocationUpstream: netwatchv1.IncidentLocation_INCIDENT_LOCATION_UPSTREAM,
}

// RunDetector feeds every stored sample to the detector and saves the
// incidents it reports, until ctx is cancelled.
func RunDetector(ctx context.Context, store *series.Store, det *incident.Detector, incidents *incident.Store, log *slog.Logger) {
	live, unsubscribe := store.Subscribe(1024)
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case smp := <-live:
			for _, inc := range det.Observe(smp) {
				if err := incidents.Save(inc, smp.At); err != nil {
					log.Error("save incident", "err", err)
					continue
				}
				if inc.Final {
					log.Info("lag incident", "id", inc.ID, "duration", inc.End.Sub(inc.Start).Round(time.Second),
						"peak_ms", inc.PeakMS, "baseline_ms", inc.BaselineMS, "suspects", len(inc.Suspects))
				}
			}
		}
	}
}

func (s *Server) ListIncidents(
	_ context.Context,
	req *connect.Request[netwatchv1.ListIncidentsRequest],
) (*connect.Response[netwatchv1.ListIncidentsResponse], error) {
	since := req.Msg.GetSince().AsDuration()
	if since <= 0 {
		since = defaultIncidentHistory
	}
	list, err := s.cfg.Incidents.List(time.Now().Add(-since))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*netwatchv1.Incident, len(list))
	for i, inc := range list {
		pb := &netwatchv1.Incident{
			Id:         inc.ID,
			Start:      timestamppb.New(inc.Start),
			Location:   incidentLocations[inc.Location],
			BaselineMs: inc.BaselineMS,
			PeakMs:     inc.PeakMS,
			LostPings:  int32(inc.Lost),
			Pings:      int32(inc.Probes),
		}
		if !inc.End.IsZero() {
			pb.End = timestamppb.New(inc.End)
		}
		for _, sus := range inc.Suspects {
			pb.Suspects = append(pb.Suspects, &netwatchv1.Suspect{
				AgentId:                 sus.Device,
				PeakUploadBytesPerSec:   sus.PeakUpBps,
				PeakDownloadBytesPerSec: sus.PeakDownBps,
			})
		}
		out[i] = pb
	}
	return connect.NewResponse(&netwatchv1.ListIncidentsResponse{Incidents: out}), nil
}
