package messaging

import (
	"context"
	"log"
	"time"
)

// PriorityProcessor consumes tasks from multiple priority streams
// and processes them in priority order (critical > high > normal > low)
type PriorityProcessor struct {
	client    *RedisClient
	agentName string
	groupName string
}

// NewPriorityProcessor creates a new priority processor for an agent
func NewPriorityProcessor(client *RedisClient, agentName string) *PriorityProcessor {
	return NewPriorityProcessorWithGroup(client, agentName, ConsumerGroupAgents)
}

// NewPriorityProcessorWithGroup creates a new priority processor with a custom consumer group
func NewPriorityProcessorWithGroup(client *RedisClient, agentName, groupName string) *PriorityProcessor {
	return &PriorityProcessor{
		client:    client,
		agentName: agentName,
		groupName: groupName,
	}
}

// Start begins processing tasks and returns a channel of tasks
func (p *PriorityProcessor) Start(ctx context.Context) <-chan *TaskMessage {
	output := make(chan *TaskMessage, 100)

	// Priority-ordered streams (critical first)
	priorities := []string{
		PriorityCritical,
		PriorityHigh,
		PriorityNormal,
		PriorityLow,
	}

	// Subscribe to each priority stream
	channels := make(map[string]<-chan Message)
	for _, priority := range priorities {
		stream := StreamName(priority, TaskTypeCoding)
		consumer := p.agentName

		msgChan, err := p.client.Subscribe(ctx, stream, p.groupName, consumer)
		if err != nil {
			log.Printf("Failed to subscribe to %s: %v", stream, err)
			continue
		}
		channels[priority] = msgChan
		log.Printf("Subscribed to stream %s as consumer %s", stream, consumer)
	}

	// Start the processing loop
	go p.processLoop(ctx, channels, output, priorities)

	return output
}

// processLoop continuously checks priority streams and forwards tasks
func (p *PriorityProcessor) processLoop(ctx context.Context, channels map[string]<-chan Message, output chan<- *TaskMessage, priorities []string) {
	defer close(output)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Priority processor shutting down for agent %s", p.agentName)
			return
		default:
			processed := false

			// Check channels in priority order
			for _, priority := range priorities {
				ch := channels[priority]
				if ch == nil {
					continue
				}

				select {
				case msg, ok := <-ch:
					if !ok {
						// Channel closed
						channels[priority] = nil
						continue
					}

					taskMsg, err := TaskMessageFromRedisValues(msg.Values)
					if err != nil {
						log.Printf("Failed to parse task message: %v", err)
						continue
					}

					// Filter for this agent (if To is set, must match agent name)
					if taskMsg.To == "" || taskMsg.To == p.agentName {
						output <- taskMsg
						processed = true
						log.Printf("Received %s priority task %s from %s", priority, taskMsg.ID, taskMsg.From)
					}

				default:
					// No message in this priority queue, check next
					continue
				}

				if processed {
					break
				}
			}

			if !processed {
				// No messages in any queue, sleep briefly to avoid CPU spinning
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

// PriorityStats holds statistics about task processing
type PriorityStats struct {
	Agent         string
	CriticalCount int64
	HighCount     int64
	NormalCount   int64
	LowCount      int64
	LastProcessed time.Time
}
