# Outbox Pattern Implementation

The Outbox pattern is a reliable messaging pattern that ensures messages are sent to a message broker even if the application crashes after committing a database transaction but before sending the message.

## Components

1. **OutboxMessage**: Represents a message stored in the outbox table
2. **OutboxStore**: Interface for persisting messages in an outbox
3. **PostgresOutboxStore**: PostgreSQL implementation of OutboxStore

## Usage

### Setup

First, create the outbox table:

```go
db := // get your database instance
outboxStore := messaging.NewPostgresOutboxStore(db)
err := outboxStore.CreateOutboxTable(ctx)
if err != nil {
    // handle error
}
```

### Saving Messages to Outbox

When you want to save a message to the outbox as part of a database transaction:

```go
err := db.Transaction(ctx, func(tx data.Transaction) error {
    // Perform your database operations
    _, err := tx.Exec(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", userID, userName)
    if err != nil {
        return err
    }
    
    // Save message to outbox
    return messaging.SaveToOutbox(ctx, tx, outboxStore, "user.created", userPayload, nil)
})
```

### Publishing Messages from Outbox

To publish pending messages from the outbox:

```go
broker := // get your message broker instance
err := messaging.PublishFromOutbox(ctx, outboxStore, broker, 100) // Process up to 100 messages
if err != nil {
    // handle error
}
```

## Best Practices

1. Run the `PublishFromOutbox` function in a background process or scheduled job
2. Consider implementing retry logic with exponential backoff for failed messages
3. Set up monitoring for the outbox table to detect stuck messages
4. Implement a cleanup process for processed messages after a certain retention period

## Error Handling

The outbox pattern includes built-in error handling:

1. Failed messages are marked with an error message
2. Retry count is incremented for each failure
3. Messages remain in the outbox until successfully processed