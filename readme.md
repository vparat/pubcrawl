# pubcrawl

Pull messages from Google Cloud Pub/Sub subscriptions, publish messages to topics, or move a subscription backlog to another topic.  Can be used to re-hydrate a topic from a deadletter subscription.

## Use with Google Cloud

In a shell with `PUBSUB_EMULATOR_HOST` unset, the CLI uses Application Default Credentials. For local access to a Google Cloud project, run `gcloud auth application-default login`. Set `--project` to your Google Cloud project ID. Resource arguments accept short IDs in that project or full names such as `projects/other-project/topics/orders`.

| Command | Required permissions |
| --- | --- |
| `pull` | `pubsub.subscriptions.consume` on the source |
| `publish` | `pubsub.topics.publish` on the destination |
| `move` | `pubsub.subscriptions.get` and `pubsub.subscriptions.consume` on the source, and `pubsub.topics.publish` on the destination |

## Command behavior

- `pull` writes to stdout by default. Use `--to-file` to create a new file. Without `--ack`, Pub/Sub can redeliver previewed messages. A preview participates in delivery and may count toward a dead-letter policy.
- `publish` reads stdin by default. Use `--from-file` for a file or `--message` for one literal payload. Use `--rate` to cap messages per second. The default, `0`, disables the cap. A failed import may already have published earlier records.
- `move` preserves payloads, attributes, and ordering keys. Pub/Sub assigns new message IDs and publish timestamps. Publishing and acknowledging are separate operations, so a crash or lost acknowledgment can cause duplicates.
- `pull` and `move` use the subscription’s current delivery state. They stop at `--limit` or after `--idle-timeout` without a new message. The default idle timeout is `5s`. Preview stops after 1,000 distinct messages by default. An explicit `--limit=-1` allows unlimited preview and retains one ID per distinct message in memory. Acknowledged pulls and moves default to unlimited. Idle time is a stopping condition. Other consumers may hold messages or receive them during the run.
- The default `lines` format represents one payload per line. Use `--format jsonl` at both ends to preserve binary or multiline payloads, attributes, and ordering keys. JSONL records contain base64-encoded `data` and optional `attributes` and `ordering_key` fields.

## Command examples

Build the CLI with `make build`. Replace `my-project`, `orders-subscription`, `orders`, and `orders-dead-letter-subscription` with IDs for resources that already exist.

Preview up to five messages from a subscription.

```sh
./pubcrawl pull orders-subscription --project my-project --limit 5
```

Publish a single payload to a topic.

```sh
./pubcrawl publish orders --project my-project --message '{"order_id":42}'
```

Publish from a file at up to 10 messages per second.

```sh
./pubcrawl publish orders --project my-project --from-file orders.jsonl --format jsonl --rate 10
```

Replay up to 100 messages from a dead-letter subscription to the original topic. The command acknowledges each source message after publishing it successfully.

```sh
./pubcrawl move orders-dead-letter-subscription orders --project my-project --limit 100
```

## Build and test

Run `make build` with Go 1.26.3 or later to build `./pubcrawl`. Run `make test` for unit tests. With the emulator running and `PUBSUB_EMULATOR_HOST` set, run `make test-integration` to exercise publish, move, and pull. The integration test checks that a failed publish leaves the source message available for retry.

## Run locally with the Pub/Sub emulator

Install the emulator with `gcloud components install pubsub-emulator`. Start it in a terminal and leave it running. The example uses a local project ID and requires no Google Cloud credentials. [Google Cloud Pub/Sub emulator guide](https://docs.cloud.google.com/pubsub/docs/emulator)

```sh
gcloud beta emulators pubsub start --project=pubcrawl-local --host-port=127.0.0.1:8085
```

If you use Docker, run the emulator with Google's Cloud CLI image. [Google Cloud CLI Docker image guide](https://docs.cloud.google.com/sdk/docs/downloads-docker)

```sh
docker run --rm -p 127.0.0.1:8085:8085 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start --project=pubcrawl-local --host-port=0.0.0.0:8085
```

In another terminal, build the CLI and point it at the emulator.

```sh
export PUBSUB_EMULATOR_HOST=127.0.0.1:8085
make build
make test-integration
```

Create local topics and subscriptions, publish a message, move it, then pull it from the destination. The `curl` requests only create the example resources. A suffix lets you rerun the example against the same emulator.

```sh
(
  set -eu
  project=pubcrawl-local
  suffix="$(date +%s)-$$"
  source="orders-dead-letter-$suffix"
  destination="orders-$suffix"
  backlog="orders-backlog-$suffix"
  delivered="orders-delivered-$suffix"
  api="http://${PUBSUB_EMULATOR_HOST:?start the emulator and set PUBSUB_EMULATOR_HOST}/v1/projects/$project"

  curl -fsS -X PUT "$api/topics/$source" -H 'Content-Type: application/json' -d '{}' >/dev/null
  curl -fsS -X PUT "$api/topics/$destination" -H 'Content-Type: application/json' -d '{}' >/dev/null
  curl -fsS -X PUT "$api/subscriptions/$backlog" -H 'Content-Type: application/json' \
    -d "{\"topic\":\"projects/$project/topics/$source\"}" >/dev/null
  curl -fsS -X PUT "$api/subscriptions/$delivered" -H 'Content-Type: application/json' \
    -d "{\"topic\":\"projects/$project/topics/$destination\"}" >/dev/null

  ./pubcrawl publish "$source" -p "$project" --message '{"order_id":42}'
  ./pubcrawl move "$backlog" "$destination" -p "$project" --limit 1
  ./pubcrawl pull "$delivered" -p "$project" --limit 1 --ack
)
```

The last command prints `{"order_id":42}`.
