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
 * Implementation of the memory-management routines for temporary double
 * precision digit vectors.
 *
 * @version $Id$
 * @ingroup dv
 */

#include <stdlib.h>
#include <errno.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_dv.h"
#include "relic_pool.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if ALLOC == DYNAMIC

void dv_new_dynam(dv_t *a, int digits) {
	int r;

	if (digits > DV_DIGS) {
		THROW(ERR_NO_PRECISION);
	}

	r = posix_memalign((void **)a, ALIGN, digits * sizeof(dig_t));
	if (r == ENOMEM || r == EINVAL) {
		THROW(ERR_NO_MEMORY);
	}
	if (r == EINVAL) {
		THROW(ERR_INVALID);
	}
}

void dv_free_dynam(dv_t *a) {
	if ((*a) != NULL) {
		free(*a);
	}
	(*a) = NULL;
}

#elif ALLOC == STATIC

void dv_new_statc(dv_t *a, int digits) {
	dig_t *t;

	if (digits > DV_DIGS) {
		THROW(ERR_NO_PRECISION);
	}

	(*a) = pool_get();
	if ((*a) == NULL) {
		THROW(ERR_NO_MEMORY);
	}
	t = (*a);
}

void dv_free_statc(dv_t *a) {
	if ((*a) != NULL) {
		pool_put((*a));
	}
	(*a) = NULL;
}

#endif
