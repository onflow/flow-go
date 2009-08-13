/*
 * Copyright 2007-2009 RELIC Project
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
 * Implementation of the SHA-256 hash function.
 *
 * @version $Id$
 * @ingroup hf
 */

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_error.h"
#include "sha.h"

static SHA256Context ctx;

void hf_map_sh256(unsigned char *hash, unsigned char *msg, int len) {
	if (SHA256Reset(&ctx) != shaSuccess) {
		THROW(ERR_INVALID);
	}
	if (SHA256Input(&ctx, msg, len) != shaSuccess) {
		THROW(ERR_INVALID);
	}
	if (SHA256Result(&ctx, hash) != shaSuccess) {
		THROW(ERR_INVALID);
	}
}
