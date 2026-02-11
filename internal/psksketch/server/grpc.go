package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	promlabels "github.com/prometheus/prometheus/model/labels"

	pb "github.com/promsketch/promsketch-dropin/api/psksketch/v1"
	"github.com/promsketch/promsketch-dropin/internal/psksketch/config"
	"github.com/promsketch/promsketch-dropin/internal/storage"
	"google.golang.org/grpc"
)

// SketchServer implements the gRPC SketchService
type SketchServer struct {
	pb.UnimplementedSketchServiceServer
	storage   *storage.Storage
	config    *config.Config
	server    *grpc.Server
	startTime time.Time
}

// NewSketchServer creates a new gRPC sketch server
func NewSketchServer(stor *storage.Storage, cfg *config.Config) *SketchServer {
	return &SketchServer{
		storage:   stor,
		config:    cfg,
		startTime: time.Now(),
	}
}

// Start starts the gRPC server
func (s *SketchServer) Start() error {
	lis, err := net.Listen("tcp", s.config.Server.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Server.ListenAddress, err)
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.Server.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.config.Server.MaxSendMsgSize),
	}

	s.server = grpc.NewServer(opts...)
	pb.RegisterSketchServiceServer(s.server, s)

	log.Printf("psksketch gRPC server listening on %s (node: %s, partitions: [%d, %d))",
		s.config.Server.ListenAddress, s.config.Node.ID,
		s.config.Node.PartitionStart, s.config.Node.PartitionEnd)

	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (s *SketchServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// Insert implements SketchService.Insert
func (s *SketchServer) Insert(ctx context.Context, req *pb.InsertRequest) (*pb.InsertResponse, error) {
	lbls := pbLabelsToPromLabels(req.Labels)
	err := s.storage.Insert(lbls, req.Timestamp, req.Value)
	if err != nil {
		return &pb.InsertResponse{Success: false, Error: err.Error()}, nil
	}
	return &pb.InsertResponse{Success: true}, nil
}

// BatchInsert implements SketchService.BatchInsert
func (s *SketchServer) BatchInsert(ctx context.Context, req *pb.BatchInsertRequest) (*pb.BatchInsertResponse, error) {
	var inserted, failed int64

	for _, ts := range req.TimeSeries {
		lbls := pbLabelsToPromLabels(ts.Labels)
		for _, sample := range ts.Samples {
			err := s.storage.Insert(lbls, sample.Timestamp, sample.Value)
			if err != nil {
				failed++
			} else {
				inserted++
			}
		}
	}

	resp := &pb.BatchInsertResponse{
		Inserted: inserted,
		Failed:   failed,
	}
	if failed > 0 {
		resp.Error = fmt.Sprintf("%d samples failed to insert", failed)
	}
	return resp, nil
}

// LookUp implements SketchService.LookUp
func (s *SketchServer) LookUp(ctx context.Context, req *pb.LookUpRequest) (*pb.LookUpResponse, error) {
	lbls := pbLabelsToPromLabels(req.Labels)
	canAnswer := s.storage.LookUp(lbls, req.FuncName, req.MinTime, req.MaxTime)
	return &pb.LookUpResponse{CanAnswer: canAnswer}, nil
}

// Eval implements SketchService.Eval
func (s *SketchServer) Eval(ctx context.Context, req *pb.EvalRequest) (*pb.EvalResponse, error) {
	lbls := pbLabelsToPromLabels(req.Labels)
	result, err := s.storage.Eval(req.FuncName, lbls, req.OtherArgs, req.MinTime, req.MaxTime, req.CurTime)
	if err != nil {
		return &pb.EvalResponse{Error: err.Error()}, nil
	}

	samples := make([]*pb.Sample, 0, len(result))
	for _, s := range result {
		samples = append(samples, &pb.Sample{
			Timestamp: s.T,
			Value:     s.F,
		})
	}

	return &pb.EvalResponse{Samples: samples}, nil
}

// Health implements SketchService.Health
func (s *SketchServer) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Healthy:       true,
		Version:       "dev",
		NodeId:        s.config.Node.ID,
		UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
	}, nil
}

// Stats implements SketchService.Stats
func (s *SketchServer) Stats(ctx context.Context, req *pb.StatsRequest) (*pb.StatsResponse, error) {
	metrics := s.storage.Metrics()
	return &pb.StatsResponse{
		NodeId:          s.config.Node.ID,
		TotalSeries:     metrics.TotalSeries,
		SketchedSeries:  metrics.SketchedSeries,
		SamplesInserted: metrics.SamplesInserted,
		PartitionStart:  int32(s.config.Node.PartitionStart),
		PartitionEnd:    int32(s.config.Node.PartitionEnd),
	}, nil
}

// pbLabelsToPromLabels converts protobuf labels to Prometheus labels
func pbLabelsToPromLabels(pbLabels []*pb.Label) promlabels.Labels {
	lbls := make(promlabels.Labels, 0, len(pbLabels))
	for _, l := range pbLabels {
		lbls = append(lbls, promlabels.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return lbls
}
