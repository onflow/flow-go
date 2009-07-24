/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the SHA-512 hash function.
 *
 * @version $Id$
 * @ingroup hf
 */

#include <arpa/inet.h>

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_error.h"
#include "sha.h"

static SHA512Context ctx;

void hf_map_sh512(unsigned char *hash, unsigned char *msg, int len) {
	if (SHA512Reset(&ctx) != shaSuccess) {
		THROW(ERR_INVALID);
	}
	if (SHA512Input(&ctx, msg, len) != shaSuccess) {
		THROW(ERR_INVALID);
	}
	if (SHA512Result(&ctx, hash) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha512_init(void) {
	if (SHA512Reset(&ctx) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha512_update(unsigned char *msg, int len) {
	if (SHA512Input(&ctx, msg, len) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha512_final(unsigned char *hash) {
	if (SHA512Result(&ctx, hash) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha512_state(unsigned char *state) {
	*(unsigned int *)(state) = htonl(ctx.Intermediate_Hash[0]);
	*(unsigned int *)(state + 4) = htonl(ctx.Intermediate_Hash[1]);
	*(unsigned int *)(state + 8) = htonl(ctx.Intermediate_Hash[2]);
	*(unsigned int *)(state + 12) = htonl(ctx.Intermediate_Hash[3]);
	*(unsigned int *)(state + 16) = htonl(ctx.Intermediate_Hash[4]);
	*(unsigned int *)(state + 20) = htonl(ctx.Intermediate_Hash[5]);
	*(unsigned int *)(state + 24) = htonl(ctx.Intermediate_Hash[6]);
	*(unsigned int *)(state + 28) = htonl(ctx.Intermediate_Hash[7]);
	*(unsigned int *)(state + 32) = htonl(ctx.Intermediate_Hash[8]);
	*(unsigned int *)(state + 36) = htonl(ctx.Intermediate_Hash[9]);
	*(unsigned int *)(state + 40) = htonl(ctx.Intermediate_Hash[10]);
	*(unsigned int *)(state + 44) = htonl(ctx.Intermediate_Hash[11]);
	*(unsigned int *)(state + 48) = htonl(ctx.Intermediate_Hash[12]);
	*(unsigned int *)(state + 52) = htonl(ctx.Intermediate_Hash[13]);
	*(unsigned int *)(state + 56) = htonl(ctx.Intermediate_Hash[14]);
	*(unsigned int *)(state + 60) = htonl(ctx.Intermediate_Hash[15]);
}
