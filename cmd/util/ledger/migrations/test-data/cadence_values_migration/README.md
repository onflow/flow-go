## Instructions for creating a snapshot

To create a new snapshot for tests, follow the below steps.

- Download a CLI which uses cadence version pre-1.0 (e.g. CLI version `v1.5.0`)

- Start emulator with the `--persist` flag.
  ```shell
  flow emulator --persist
  ```

- Create an account name `test`.
  ```shell
  flow accounts create
  ```

- Update the `testAccountAddress` constant in the [cadence_values_migration_test.go](../../cadence_values_migration_test.go)
  with the address of the account just created.

- Run the transaction [store_transaction.cdc](store_transaction.cdc), using the newly created `test` account as the signer.
  ```shell
  flow transactions send store_transaction.cdc --signer test
  ```

- Create a snapshot os the emulator state using the REST API:
  ```shell
  curl -H "Content-type: application/x-www-form-urlencoded" -X POST http://localhost:8080/emulator/snapshots -d "name=test_snapshot"
  ```

- Above will create file named `snapshot.test_snapshot` in a directly called `flowdb` where the flow project was initialized.
  Copy it to this directly (`test-data/cadence_values_migration`) and rename it to `snapshot`.
