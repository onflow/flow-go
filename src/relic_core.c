/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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
 * Implementation of the library basic functions.
 *
 * @version $Id$
 * @ingroup relic
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_rand.h"
#include "relic_types.h"
#include "relic_error.h"
#include "relic_fp.h"
#include "relic_fb.h"
#include "relic_ft.h"
#include "relic_ep.h"
#include "relic_eb.h"
#include "relic_cp.h"
#include "relic_pp.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

ctx_t core_ctx[1];

int core_init(void) {
#if defined(CHECK) || defined(TRACE)
	core_ctx->trace = 0;
#endif

#ifdef CHECK
	core_ctx->reason = (char **)malloc(ERR_MAX * sizeof(char *));
	if (core_ctx->reason == NULL) {
		return STS_ERR;
	} else {
		core_ctx->reason[ERR_NO_MEMORY] = MSG_NO_MEMORY;
		core_ctx->reason[ERR_NO_PRECISION] = MSG_NO_PRECISION;
		core_ctx->reason[ERR_NO_FILE] = MSG_NO_FILE;
		core_ctx->reason[ERR_NO_READ] = MSG_NO_READ;
		core_ctx->reason[ERR_INVALID] = MSG_INVALID;
		core_ctx->reason[ERR_NO_BUFFER] = MSG_NO_BUFFER;
		core_ctx->reason[ERR_NO_FIELD] = MSG_NO_FIELD;
		core_ctx->reason[ERR_NO_CURVE] = MSG_NO_CURVE;
	}
	core_ctx->code = STS_OK;
	core_ctx->last = NULL;
#endif /* CHECK */

	TRY {
		rand_init();
#ifdef WITH_FP
		fp_prime_init();
#endif
#ifdef WITH_FB
		fb_poly_init();
#endif
#ifdef WITH_FT
		ft_poly_init();
#endif
#ifdef WITH_EP
		ep_curve_init();
#endif
#ifdef WITH_EB
		eb_curve_init();
#endif
#ifdef WITH_PP
		pp_map_init();
#endif
	}
	CATCH_ANY {
		return STS_ERR;
	}

	return STS_OK;
}

int core_clean(void) {
	rand_clean();
#ifdef WITH_FP
	fp_prime_clean();
#endif
#ifdef WITH_FB
	fb_poly_clean();
#endif
#ifdef WITH_FT
	ft_poly_clean();
#endif
#ifdef WITH_EP
	ep_curve_clean();
#endif
#ifdef WITH_EB
	eb_curve_clean();
#endif
#ifdef WITH_PP
		pp_map_clean();
#endif

#ifdef CHECK
	free(core_ctx->reason);
#endif
	return STS_OK;
}

