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
 * Tests for configuration management.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

int main(void) {
	int code = STS_ERR;

	/* Initialize library with default configuration. */
	core_init();

	util_banner("Tests for the CORE module:\n", 0);

	TEST_ONCE("the library context is consistent") {
		TEST_ASSERT(core_get() == core_ctx, end);
	} TEST_END;

	TEST_ONCE("switching the library context is correct") {
		ctx_t new_ctx, *old_ctx;
		/* Backup the old context. */
		old_ctx = core_get();
		/* Switch the library context. */
		core_set(&new_ctx);
		/* Reinitialize library with new context. */
		core_init();
		/* Run function to manipulate the library context. */
		THROW(ERR_NO_MEMORY);
		core_set(old_ctx);
		TEST_ASSERT(err_get_code() == STS_OK, end);
		core_set(&new_ctx);
		TEST_ASSERT(err_get_code() == STS_ERR, end);
		/* Now we need to finalize the new context. */
		core_clean();
		/* And restore the original context. */
		core_set(old_ctx);
	} TEST_END;

	util_banner("All tests have passed.\n", 0);

	code = STS_OK;
  end:
	core_clean();
	if (code == STS_ERR)
		return 0;
	else {
		return 1;
	}
}
