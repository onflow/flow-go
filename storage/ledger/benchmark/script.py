import json


chunk_update_delay = []
ptrie_const_delay = []
read_delay = []
read_with_proof_delay = []
storage = []

with open("logs.txt") as inp:
	for line in inp:
		if line.strip():
			data = json.loads(line.strip())
			if data.get("message", "") == "update register":
				chunk_update_delay += [data.get("update_time_ms")] 
				storage += [data.get("storage_size_mb")]
			elif data.get("message", "") == "read register":
				read_delay += [data.get("read_time_ms")]
			elif data.get("message", "") == "read register with proof":
				read_with_proof_delay += [data.get("read_time_ms")]
				ptrie_const_delay += [data.get("time_to_const_psmt_ms")]


# print(storage)
# print(chunk_update_delay)
# print(read_delay)
# print(ptrie_const_delay)
print(read_with_proof_delay)

