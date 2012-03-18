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
 * Benchmarks for manipulating temporary double-precision digit vectors.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void memory(void) {
	dv_t a[BENCH];

	BENCH_SMALL("dv_null", dv_null(a[i]));

	BENCH_SMALL("dv_new", dv_new(a[i]));
	for (int i = 0; i < BENCH; i++) {
		dv_free(a[i]);
	}

	for (int i = 0; i < BENCH; i++) {
		dv_new(a[i]);
	}
	BENCH_SMALL("dv_free", dv_free(a[i]));

	(void)a;
}

int main(void) {
	core_init();
	conf_print();
	util_banner("Benchmarks for the DV module:\n", 0);
	util_banner("Utilities:\n", 0);
	memory();
	core_clean();
	return 0;
}
