/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the core functions for computing pairings over binary
 * fields.
 *
 * @version $Id$
 * @ingroup pb
 */

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_bench.h"
#include "relic_pb.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void pb_map_init() {
	ctx_t *ctx = core_get();

	for (int i = 0; i < FB_TABLE; i++) {
		ctx->pb_ptr[i] = &(ctx->pb_exp[i]);
	}

#if PB_MAP == ETATS || PB_MAP == ETATN
	fb_itr_pre((fb_t *)pb_map_get_tab(), 4 * (((FB_BITS + 1) / 2) / 4));
#endif

#if PB_MAP == ETAT2 || PB_MAP == OETA2
	fb_itr_pre(pb_map_get_tab(), 6 * (((FB_BITS - 1) / 2) / 6));
#endif

#if defined(PB_PARAL) && (PB_MAP == ETATS || PB_MAP == ETATN)

	int chunk = (int)ceilf((FB_BITS - 1) / (2.0 * CORES));

	for (int i = 0; i < CORES; i++) {
		for (int j = 0; j < FB_TABLE; j++) {
			ctx->pb_pow[0][i][j] = &(ctx->pb_sqr[i][j]);
			ctx->pb_pow[1][i][j] = &(ctx->pb_srt[i][j]);
		}
	}

	for (int i = 0; i < CORES; i++) {
		fb_itr_pre(pb_map_get_sqr(i), i * chunk);
		fb_itr_pre(pb_map_get_srt(i), -i * chunk);
	}

#endif
}

void pb_map_clean() {

}

const fb_t *pb_map_get_tab() {
#if ALLOC == AUTO
	return (const fb_t *)*core_get()->pb_ptr;
#else
	return (const fb_t *)core_get()->pb_ptr;
#endif
}

const fb_t *pb_map_get_sqr(int core) {
#ifdef PB_PARAL

#if ALLOC == AUTO
	return (const fb_t *)*core_get()->pb_pow[0][core];
#else
	return (const fb_t *)core_get()->pb_pow[0][core];
#endif

#else
	return NULL;
#endif
}

const fb_t *pb_map_get_srt(int core) {
#ifdef PB_PARAL

#if ALLOC == AUTO
	return (const fb_t *)*core_get()->pb_pow[1][core];
#else
	return (const fb_t *)core_get()->pb_pow[1][core];
#endif

#else
	return NULL;
#endif
}

int pb_map_get_par(int core) {
	int chunk = (int)ceilf((FB_BITS - 1) / (2.0 * CORES));

	if (core == CORES) {
		return MIN((FB_BITS - 1) / 2, core * chunk);
	} else {
		return core * chunk;
	}
}
