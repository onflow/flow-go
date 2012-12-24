/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Tests for manipulating temporary double-precision digit vectors.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

static int memory(void) {
	err_t e;
	int code = STS_ERR;

#if ALLOC == STATIC
	dv_t a[POOL_SIZE + 1];

	for (int i = 0; i < POOL_SIZE; i++) {
		dv_null(a[i]);
	}
	dv_null(a[POOL_SIZE]);

	TRY {
		TEST_BEGIN("all memory can be allocated") {
			for (int j = 0; j < POOL_SIZE; j++) {
				dv_new(a[j]);
			}
			dv_new(a[POOL_SIZE]);
		}
	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				for (int j = 0; j < POOL_SIZE; j++) {
					dv_free(a[j]);
				}
				dv_free(a[POOL_SIZE]);
				break;
		}
	}
#else
	dv_t a;

	dv_null(a);

	TRY {
		TEST_BEGIN("temporary memory can be allocated") {
			dv_new(a);
			dv_free(a);
		} TEST_END;
	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				util_print("FATAL ERROR!\n");
				ERROR(end);
				break;
		}
	}
#endif
	code = STS_OK;
  end:
	return code;
}

int main(void) {
	core_init();

	util_banner("Tests for the DV module:\n", 0);

	if (memory() != STS_OK) {
		core_clean();
		return 1;
	}

	util_banner("All tests have passed.\n", 0);

	core_clean();
	return 0;
}
