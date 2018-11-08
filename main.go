package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"
)

func main() {
	// parse cli args
	var (
		flagP  = flag.String("p", "", "Project id")
		flagT  = flag.String("t", "", "Topic id")
		flagS  = flag.String("s", "", "Subscription id")
		flagSE = flag.String("e", "", "Subscription endpoint")
	)
	flag.Parse()

	// set init values
	ctx := context.Background()
	projectID := *flagP
	if projectID == "" {
		fmt.Fprintf(os.Stderr, "Project id must to set.")
		os.Exit(1)
	}
	topicID := *flagT
	if topicID == "" {
		fmt.Fprintf(os.Stderr, "Topic id must be set.")
		os.Exit(1)
	}
	subID := *flagS
	if subID == "" {
		fmt.Fprintf(os.Stderr, "Sub id must be set.")
		os.Exit(1)
	}
	subEndpoint := *flagSE
	if subEndpoint == "" {
		fmt.Fprintf(os.Stderr, "Sub endpoint must be set.")
		os.Exit(1)
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Could not create pubsub Client: %v", err)
	}

	// Print all the subscriptions in the project.
	fmt.Println("Listing all subscriptions from the project:" + projectID)
	subs, err := list(client)
	if err != nil {
		log.Fatal(err)
	}
	for _, sub := range subs {
		fmt.Println(sub)
	}

	t := createTopicIfNotExists(topicID, client)

	// Create a new subscription.
	if err := createWithEndpoint(client, subID, t, subEndpoint); err != nil {
		log.Fatal(err)
	}
}

func list(client *pubsub.Client) ([]*pubsub.Subscription, error) {
	ctx := context.Background()
	// [START pubsub_list_subscriptions]
	var subs []*pubsub.Subscription
	it := client.Subscriptions(ctx)
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	// [END pubsub_list_subscriptions]
	return subs, nil
}

func pullMsgs(client *pubsub.Client, subName string, topic *pubsub.Topic) error {
	ctx := context.Background()

	// Publish 10 messages on the topic.
	var results []*pubsub.PublishResult
	for i := 0; i < 10; i++ {
		res := topic.Publish(ctx, &pubsub.Message{
			Data: []byte(fmt.Sprintf("hello world #%d", i)),
		})
		results = append(results, res)
	}

	// Check that all messages were published.
	for _, r := range results {
		_, err := r.Get(ctx)
		if err != nil {
			return err
		}
	}

	// [START pubsub_subscriber_async_pull]
	// [START pubsub_quickstart_subscriber]
	// Consume 10 messages.
	var mu sync.Mutex
	received := 0
	sub := client.Subscription(subName)
	cctx, cancel := context.WithCancel(ctx)
	err := sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		msg.Ack()
		fmt.Printf("Got message: %s\n", string(msg.Data))
		mu.Lock()
		defer mu.Unlock()
		received++
		if received == 10 {
			cancel()
		}
	})
	if err != nil {
		return err
	}
	// [END pubsub_subscriber_async_pull]
	// [END pubsub_quickstart_subscriber]
	return nil
}

func pullMsgsError(client *pubsub.Client, subName string) error {
	ctx := context.Background()
	// [START pubsub_subscriber_error_listener]
	// If the service returns a non-retryable error, Receive returns that error after
	// all of the outstanding calls to the handler have returned.
	err := client.Subscription(subName).Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		fmt.Printf("Got message: %s\n", string(msg.Data))
		msg.Ack()
	})
	if err != nil {
		return err
	}
	// [END pubsub_subscriber_error_listener]
	return nil
}

func pullMsgsSettings(client *pubsub.Client, subName string) error {
	ctx := context.Background()
	// [START pubsub_subscriber_flow_settings]
	sub := client.Subscription(subName)
	sub.ReceiveSettings.MaxOutstandingMessages = 10
	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		fmt.Printf("Got message: %s\n", string(msg.Data))
		msg.Ack()
	})
	if err != nil {
		return err
	}
	// [END pubsub_subscriber_flow_settings]
	return nil
}

func create(client *pubsub.Client, subName string, topic *pubsub.Topic) error {
	ctx := context.Background()
	// [START pubsub_create_pull_subscription]
	sub, err := client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
		Topic:       topic,
		AckDeadline: 20 * time.Second,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Created subscription: %v\n", sub)
	// [END pubsub_create_pull_subscription]
	return nil
}

func createWithEndpoint(client *pubsub.Client, subName string, topic *pubsub.Topic, endpoint string) error {
	ctx := context.Background()
	// [START pubsub_create_push_subscription]

	// For example, endpoint is "https://my-test-project.appspot.com/push".
	sub, err := client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
		Topic:       topic,
		AckDeadline: 10 * time.Second,
		PushConfig:  pubsub.PushConfig{Endpoint: endpoint},
	})
	if err != nil {
		return err
	}
	fmt.Printf("Created subscription: %v\n", sub)
	// [END pubsub_create_push_subscription]
	return nil
}

func updateEndpoint(client *pubsub.Client, subName string, endpoint string) error {
	ctx := context.Background()
	// [START pubsub_update_push_configuration]

	// For example, endpoint is "https://my-test-project.appspot.com/push".
	subConfig, err := client.Subscription(subName).Update(ctx, pubsub.SubscriptionConfigToUpdate{
		PushConfig: &pubsub.PushConfig{Endpoint: endpoint},
	})
	if err != nil {
		return err
	}
	fmt.Printf("Updated subscription config: %#v", subConfig)
	// [END pubsub_update_push_configuration]
	return nil
}

func delete(client *pubsub.Client, subName string) error {
	ctx := context.Background()
	// [START pubsub_delete_subscription]
	sub := client.Subscription(subName)
	if err := sub.Delete(ctx); err != nil {
		return err
	}
	fmt.Println("Subscription deleted.")
	// [END pubsub_delete_subscription]
	return nil
}

func createTopicIfNotExists(id string, c *pubsub.Client) *pubsub.Topic {
	ctx := context.Background()

	topic := id
	// Create a topic to subscribe to.
	t := c.Topic(topic)
	ok, err := t.Exists(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if ok {
		return t
	}

	t, err = c.CreateTopic(ctx, topic)
	if err != nil {
		log.Fatalf("Failed to create the topic: %v", err)
	}
	return t
}

func getPolicy(c *pubsub.Client, subName string) (*iam.Policy, error) {
	ctx := context.Background()

	// [START pubsub_get_subscription_policy]
	policy, err := c.Subscription(subName).IAM().Policy(ctx)
	if err != nil {
		return nil, err
	}
	for _, role := range policy.Roles() {
		log.Printf("%q: %q", role, policy.Members(role))
	}
	// [END pubsub_get_subscription_policy]
	return policy, nil
}

func addUsers(c *pubsub.Client, subName string) error {
	ctx := context.Background()

	// [START pubsub_set_subscription_policy]
	sub := c.Subscription(subName)
	policy, err := sub.IAM().Policy(ctx)
	if err != nil {
		return err
	}
	// Other valid prefixes are "serviceAccount:", "user:"
	// See the documentation for more values.
	policy.Add(iam.AllUsers, iam.Viewer)
	policy.Add("group:cloud-logs@google.com", iam.Editor)
	if err := sub.IAM().SetPolicy(ctx, policy); err != nil {
		return err
	}
	// NOTE: It may be necessary to retry this operation if IAM policies are
	// being modified concurrently. SetPolicy will return an error if the policy
	// was modified since it was retrieved.
	// [END pubsub_set_subscription_policy]
	return nil
}

func testPermissions(c *pubsub.Client, subName string) ([]string, error) {
	ctx := context.Background()

	// [START pubsub_test_subscription_permissions]
	sub := c.Subscription(subName)
	perms, err := sub.IAM().TestPermissions(ctx, []string{
		"pubsub.subscriptions.consume",
		"pubsub.subscriptions.update",
	})
	if err != nil {
		return nil, err
	}
	for _, perm := range perms {
		log.Printf("Allowed: %v", perm)
	}
	// [END pubsub_test_subscription_permissions]
	return perms, nil
}