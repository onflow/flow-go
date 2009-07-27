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
 * Implementation of the SHA-1 hash function.
 *
 * @version $Id$
 * @ingroup hf
 */

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_error.h"
#include "relic_util.h"
#include "sha.h"

static SHA1Context ctx;

void hf_map_shone(unsigned char *hash, unsigned char *msg, int len) {
	if (SHA1Reset(&ctx) != shaSuccess) {
		THROW(ERR_INVALID);
	}
	if (SHA1Input(&ctx, msg, len) != shaSuccess) {
		THROW(ERR_INVALID);
	}
	if (SHA1Result(&ctx, hash) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha1_init(void) {
	if (SHA1Reset(&ctx) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha1_update(unsigned char *msg, int len) {
	if (SHA1Input(&ctx, msg, len) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha1_final(unsigned char *hash) {
	if (SHA1Result(&ctx, hash) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}

void hf_sha1_state(unsigned char *state) {
	*(unsigned int *)(state) = util_conv_big(ctx.Intermediate_Hash[0]);
	*(unsigned int *)(state + 4) = util_conv_big(ctx.Intermediate_Hash[1]);
	*(unsigned int *)(state + 8) = util_conv_big(ctx.Intermediate_Hash[2]);
	*(unsigned int *)(state + 12) = util_conv_big(ctx.Intermediate_Hash[3]);
	*(unsigned int *)(state + 16) = util_conv_big(ctx.Intermediate_Hash[4]);
}
