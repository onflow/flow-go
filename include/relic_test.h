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
 * @defgroup tests Automated tests.
 */

/**
 * @file
 *
 * Useful routines for testing.
 *
 * @version $Id: relic_test.h 3 2009-04-08 01:05:51Z dfaranha $
 * @ingroup test
 */

#ifndef RELIC_TEST_H
#define RELIC_TEST_H

#include <string.h>

#include "relic_conf.h"

/**
 * Runs a new benchmark once.
 *
 * @param[in] LABEL			- the label for this benchmark.
 */
#define TEST_ONCE(P)														\
	util_print("Testing if %s... %*s", P, (int)(64 - strlen(P)), " ");		\

/**
 * Tests a sequence of commands to see if they respect some property.
 *
 * @param[in] P				- the description of the property.
 */
#define TEST_BEGIN(P)														\
	util_print("Testing if %s... %*s", P, (int)(64 - strlen(P)), " ");		\
	for (int i = 0; i < TESTS; i++)											\

/**
 * Asserts a condition.
 *
 * If the condition is not satisfied, a unconditional jump is made to the passed
 * label.
 *
 * @param[in] C				- the condition to assert.
 * @param[in] LABEL			- the label to jump if the condition is no satisfied.
 */
#define TEST_ASSERT(C, LABEL)												\
		if (!(C)) {															\
			test_print_fail();												\
			ERROR(LABEL);													\
		}

/**
 * Finalizes a test printing the test result.
 */
#define TEST_END															\
		test_print_pass()													\

/**
 * Prints a string indicating that the test failed.
 */
void test_print_fail(void);

/**
 * Prints a string indicating that the test passed.
 */
void test_print_pass(void);

#endif /* !RELIC_TEST_H */
