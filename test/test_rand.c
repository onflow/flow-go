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
 * Tests for random number generation.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

unsigned char test[20] = {
	0xBD, 0x02, 0x9B, 0xBE, 0x7F, 0x51, 0x96, 0x0B, 0xCF, 0x9E,
	0xDB, 0x2B, 0x61, 0xF0, 0x6F, 0x0F, 0xEB, 0x5A, 0x38, 0xB6
};

unsigned char result[40] = {
	0x20, 0x70, 0xb3, 0x22, 0x3D, 0xBA, 0x37, 0x2F, 0xDE, 0x1C,
	0x0F, 0xFC, 0x7B, 0x2E, 0x3B, 0x49, 0x8B, 0x26, 0x06, 0x14,
	0x3C, 0x6C, 0x18, 0xBA, 0xCB, 0x0F, 0x6C, 0x55, 0xBA, 0xBB,
	0x13, 0x78, 0x8E, 0x20, 0xD7, 0x37, 0xA3, 0x27, 0x51, 0x16
};

static int gen(void) {
	int code = STS_ERR;
	unsigned char out[40];

	TEST_ONCE("random generator is correct") {
		rand_seed(test, 20);
		rand_bytes(out, 40);
		TEST_ASSERT(memcmp(out, result, 40) == 0, end);
	}
	TEST_END;

	code = STS_OK;

  end:
	return code;
}

int main(void) {
	core_init();

	util_banner("Tests for the RAND module:\n", 0);

	if (gen() != STS_OK) {
		core_clean();
		return 1;
	}

	util_banner("All tests have passed.\n", 0);

	core_clean();
	return 0;
}
