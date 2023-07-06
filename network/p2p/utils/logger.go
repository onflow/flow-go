package utils

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
)

// TopicScoreParamsLogger is a helper function that returns a logger with the topic score params added as fields.
// Args:
// logger: zerolog.Logger - logger to add fields to
// topicName: string - name of the topic
// params: pubsub.TopicScoreParams - topic score params
func TopicScoreParamsLogger(logger zerolog.Logger, topicName string, topicParams *pubsub.TopicScoreParams) zerolog.Logger {
	return logger.With().Str("topic", topicName).
		Bool("atomic_validation", topicParams.SkipAtomicValidation).
		Float64("topic_weight", topicParams.TopicWeight).
		Float64("time_in_mesh_weight", topicParams.TimeInMeshWeight).
		Dur("time_in_mesh_quantum", topicParams.TimeInMeshQuantum).
		Float64("time_in_mesh_cap", topicParams.TimeInMeshCap).
		Float64("first_message_deliveries_weight", topicParams.FirstMessageDeliveriesWeight).
		Float64("first_message_deliveries_decay", topicParams.FirstMessageDeliveriesDecay).
		Float64("first_message_deliveries_cap", topicParams.FirstMessageDeliveriesCap).
		Float64("mesh_message_deliveries_weight", topicParams.MeshMessageDeliveriesWeight).
		Float64("mesh_message_deliveries_decay", topicParams.MeshMessageDeliveriesDecay).
		Float64("mesh_message_deliveries_cap", topicParams.MeshMessageDeliveriesCap).
		Float64("mesh_message_deliveries_threshold", topicParams.MeshMessageDeliveriesThreshold).
		Dur("mesh_message_deliveries_window", topicParams.MeshMessageDeliveriesWindow).
		Dur("mesh_message_deliveries_activation", topicParams.MeshMessageDeliveriesActivation).
		Float64("mesh_failure_penalty_weight", topicParams.MeshFailurePenaltyWeight).
		Float64("mesh_failure_penalty_decay", topicParams.MeshFailurePenaltyDecay).
		Float64("invalid_message_deliveries_weight", topicParams.InvalidMessageDeliveriesWeight).
		Float64("invalid_message_deliveries_decay", topicParams.InvalidMessageDeliveriesDecay).Logger()
}
