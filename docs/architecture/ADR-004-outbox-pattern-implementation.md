# ADR-004: Outbox Pattern for Reliable Messaging

## Status

Accepted

## Context

Microservices often need to update a database and publish a message atomically. Without proper coordination, this can lead to inconsistent state where the database is updated but the message is not published, or vice versa. This is particularly problematic in event-driven architectures.

Key requirements:
- Atomic database updates and message publishing
- Guaranteed message delivery (at-least-once semantics)
- Support for different message brokers (RabbitMQ, Kafka)
- Transparent implementation that doesn't complicate business logic
- Resilience to broker outages

## Decision

We will implement the Transactional Outbox pattern with the following components:

### 1. Outbox Table
A database table that stores messages to be published:

```sql
CREATE TABLE outbox_messages (
    id UUID PRIMARY KEY,
    topic VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    processed_at TIMESTAMP NULL
);
```

### 2. Outbox Store Interface
An interface for managing outbox messages:

```go
type OutboxStore interface {
    SaveMessage(ctx context.Context, tx Transaction, message OutboxMessage) error
    GetPendingMessages(ctx context.Context, limit int) ([]OutboxMessage, error)
    MarkProcessed(ctx context.Context, messageID string) error
}
```

### 3. Message Broker Abstraction
A unified interface for different message brokers:

```go
type Broker interface {
    Publish(ctx context.Context, topic string, message Message) error
    Subscribe(topic string, handler MessageHandler) error
}
```

### 4. Relay Process
A background process that:
- Polls the outbox table for unprocessed messages
- Publishes messages to the actual broker
- Marks messages as processed on successful delivery
- Implements exponential backoff for failed deliveries

### 5. Transparent Integration
The broker implementation automatically uses the outbox when within a database transaction:

```go
// This automatically uses the outbox pattern
err := db.Transaction(ctx, func(tx Transaction) error {
    user, err := userRepo.Create(ctx, tx, userData)
    if err != nil {
        return err
    }
    
    // Message is saved to outbox table within the same transaction
    return broker.Publish(ctx, "user.created", UserCreatedEvent{
        UserID: user.ID,
        Email:  user.Email,
    })
})
```

## Consequences

### Positive
- Guaranteed consistency between database and messaging
- Resilience to message broker outages
- Transparent implementation - no changes to business logic
- At-least-once delivery semantics
- Support for multiple message brokers
- Audit trail of all published messages

### Negative
- Additional database table and storage overhead
- Potential message duplication (at-least-once semantics)
- Background relay process adds operational complexity
- Slight delay between database commit and message delivery
- Additional database queries for message management

## Alternatives Considered

### 1. Two-Phase Commit (2PC)
Use distributed transactions between database and message broker.
- **Rejected**: Complex to implement, poor performance, not supported by all brokers

### 2. Saga Pattern
Use compensating transactions to handle failures.
- **Rejected**: More complex than needed for this use case, doesn't guarantee atomicity

### 3. Event Sourcing
Store events as the primary data model.
- **Rejected**: Significant architectural change, not suitable for all use cases

### 4. Message Broker Transactions
Use broker-native transaction support.
- **Rejected**: Not all brokers support transactions, creates broker lock-in

### 5. Dual Writes
Write to database and publish message separately.
- **Rejected**: Cannot guarantee atomicity, leads to inconsistent state