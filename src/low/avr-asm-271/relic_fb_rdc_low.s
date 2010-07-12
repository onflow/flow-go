/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009, 2010 RELIC Authors
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
 * Implementation of the low-level binary field modular reduction functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

.arch atmega128

#if FB_POLYN == 163

.global fb_rdcn_table0

.data

/*
 * This table will be loaded on address 0x100.
 */
fb_rdcn_table0:
.byte	0x00, 0x19, 0x32, 0x2b, 0x64, 0x7d, 0x56, 0x4f, 0xc9, 0xd0, 0xfb, 0xe2, 0xad, 0xb4, 0x9f, 0x86,	\
		0x92, 0x8b, 0xa0, 0xb9, 0xf6, 0xef, 0xc4, 0xdd,	0x5b, 0x42, 0x69, 0x70, 0x3f, 0x26, 0x0d, 0x14,	\
		0x24, 0x3d, 0x16, 0x0f, 0x40, 0x59, 0x72, 0x6b,	0xed, 0xf4, 0xdf, 0xc6, 0x89, 0x90, 0xbb, 0xa2,	\
		0xb6, 0xaf, 0x84, 0x9d, 0xd2, 0xcb, 0xe0, 0xf9,	0x7f, 0x66, 0x4d, 0x54, 0x1b, 0x02, 0x29, 0x30,	\
		0x48, 0x51, 0x7a, 0x63, 0x2c, 0x35, 0x1e, 0x07,	0x81, 0x98, 0xb3, 0xaa, 0xe5, 0xfc, 0xd7, 0xce,	\
		0xda, 0xc3, 0xe8, 0xf1, 0xbe, 0xa7, 0x8c, 0x95,	0x13, 0x0a, 0x21, 0x38, 0x77, 0x6e, 0x45, 0x5c,	\
		0x6c, 0x75, 0x5e, 0x47, 0x08, 0x11, 0x3a, 0x23,	0xa5, 0xbc, 0x97, 0x8e, 0xc1, 0xd8, 0xf3, 0xea,	\
		0xfe, 0xe7, 0xcc, 0xd5, 0x9a, 0x83, 0xa8, 0xb1,	0x37, 0x2e, 0x05, 0x1c, 0x53, 0x4a, 0x61, 0x78,	\
		0x90, 0x89, 0xa2, 0xbb, 0xf4, 0xed, 0xc6, 0xdf,	0x59, 0x40, 0x6b, 0x72, 0x3d, 0x24, 0x0f, 0x16,	\
		0x02, 0x1b, 0x30, 0x29, 0x66, 0x7f, 0x54, 0x4d,	0xcb, 0xd2, 0xf9, 0xe0, 0xaf, 0xb6, 0x9d, 0x84,	\
		0xb4, 0xad, 0x86, 0x9f, 0xd0, 0xc9, 0xe2, 0xfb,	0x7d, 0x64, 0x4f, 0x56, 0x19, 0x00, 0x2b, 0x32,	\
		0x26, 0x3f, 0x14, 0x0d, 0x42, 0x5b, 0x70, 0x69,	0xef, 0xf6, 0xdd, 0xc4, 0x8b, 0x92, 0xb9, 0xa0,	\
		0xd8, 0xc1, 0xea, 0xf3, 0xbc, 0xa5, 0x8e, 0x97,	0x11, 0x08, 0x23, 0x3a, 0x75, 0x6c, 0x47, 0x5e,	\
		0x4a, 0x53, 0x78, 0x61, 0x2e, 0x37, 0x1c, 0x05,	0x83, 0x9a, 0xb1, 0xa8, 0xe7, 0xfe, 0xd5, 0xcc,	\
		0xfc, 0xe5, 0xce, 0xd7, 0x98, 0x81, 0xaa, 0xb3,	0x35, 0x2c, 0x07, 0x1e, 0x51, 0x48, 0x63, 0x7a,	\
		0x6e, 0x77, 0x5c, 0x45, 0x0a, 0x13, 0x38, 0x21,	0xa7, 0xbe, 0x95, 0x8c, 0xc3, 0xda, 0xf1, 0xe8

/*
 * This table will be loaded on address 0x200.
 */
fb_rdcn_table1:
.byte	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, \
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,	0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, \
		0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, \
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, \
		0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, \
		0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, \
		0x05, 0x05, 0x05, 0x05, 0x05, 0x05, 0x05, 0x05,	0x05, 0x05, 0x05, 0x05, 0x05, 0x05, 0x05, 0x05, \
		0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04,	0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, \
		0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, \
		0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, 0x0d, \
		0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, \
		0x0e, 0x0e, 0x0e, 0x0e, 0x0e, 0x0e, 0x0e, 0x0e,	0x0e, 0x0e, 0x0e, 0x0e, 0x0e, 0x0e, 0x0e, 0x0e, \
		0x0a, 0x0a, 0x0a, 0x0a, 0x0a, 0x0a, 0x0a, 0x0a,	0x0a, 0x0a, 0x0a, 0x0a, 0x0a, 0x0a, 0x0a, 0x0a, \
		0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,	0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, \
		0x09, 0x09, 0x09, 0x09, 0x09, 0x09, 0x09, 0x09,	0x09, 0x09, 0x09, 0x09, 0x09, 0x09, 0x09, 0x09, \
		0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08

.text

.global fb_rdcn_low

.macro R0 r0, r1, r2
	ld		\r1, x

	inc		r27
	ld		\r0, x
	dec		r27

	swap	r26
	andi	r26, 0xF0
	lsl		r26
	mov		\r2, r26
.endm

.macro R r0, r1, r2, d1, d2
	ld		r23, x
	eor		\r1, r23

	inc		r27
	ld		r23, x
	dec		r27
	eor		\r0, r23

	swap	r26
	andi	r26, 0xF0
	lsl		r26
	mov		\r2, r26
.endm

.macro step1 d1,d2
	ldd		r26,y+\d1
	R r21, r22, r20
	ldd		r23,y+\d2
	eor		r23,r21
	std		z+\d2,r23
.endm

.macro step2 d1,d2
	ldd		r26,y+\d1
	R r22, r20, r21
	ldd		r23,y+\d2
	eor		r23,r22
	std		z+\d2,r23
.endm

.macro step3 d1,d2
	ldd		r26,y+\d1
	R r20, r21, r22
	ldd		r23,y+\d2
	eor		r23,r20
	std		z+\d2,r23
.endm

fb_rdcn_low:
	push	r28
	push	r29

	movw	r30,r24				; Copy c to z
	movw	r28,r22				; Copy a to y

	clr		r26
	ldi		r27, hi8(fb_rdcn_table0)

	ldd		r26, y+40
	R0 		r21, r22, r20
	ldd		r23, y+21
	eor		r23, r21
	mov		r25, r23			; We cannot write to c[21].

	ldd		r26, y+39
	R		r22, r20, r21
	ldd		r18, y + 20
	eor		r18, r22

	step3 38,19
	step1 37,18
	step2 36,17
	step3 35,16
	step1 34,15
	step2 33,14
	step3 32,13
	step1 31,12
	step2 30,11
	step3 29,10
	step1 28,9
	step2 27,8
	step3 26,7
	step1 25,6
	step2 24,5
	step3 23,4
	step1 22,3

	mov		r26, r25
	R 		r22, r20, r21
	ldd		r23, y + 2
	eor		r23, r22
	std		z + 2, r23

	ldd 	r24, y + 1
	eor 	r24, r20		; r24 = m[1]

	ldd		r25, y + 0
	eor		r25, r21		; r25 = m[0]

	mov		r20, r18
	lsr		r20
	swap	r20
	andi	r20, 0xC0	; r20 = (t>>3)<<6

	mov		r19, r20
	lsl		r19			; r19 = (t>>3)<<7

	mov		r21, r18
	andi	r21, 0xF8		; r21 = (t>>3)<<3

	mov		r22, r18
	lsr		r22
	lsr		r22
	lsr		r22			; r22 = (t >> 3)

	eor		r22, r19
	eor		r22, r20
	eor		r22, r21

	eor		r25, r22
	std		z + 0, r25

	mov		r25, r18
	swap	r25
	andi	r25, 0x0F
	mov		r23, r25
	lsr		r23
	eor		r23, r25

	eor		r24, r23
	std		z + 1, r24

	andi	r18, 0x07
	std		z + 20, r18

	pop		r29
	pop		r28
	ret
#endif

#if FB_POLYN == 233

.global fb_rdcn_low

.macro LOAD i
	ldd		r18, y + \i			; t0 = r18
	ldd		r19, y + (\i - 1)	; t1 = r19
	bst		r19, 0
	bld		r20, 7		; r20 = t1 << 7
	mov		r22, r18
	lsr		r22			; r22 = t0 >> 1
	mov		r21, r19
	ror		r21			; r21 = (t0 << 7) ^ (t1 >> 1)
	mov		r23, r19
	lsl		r23			; r23 = t1 << 1
	bst		r19, 7
	lsl		r18
	bld		r18, 0		; r18 = (t0 << 1) ^ (t1 >> 7)
	clr		r19
	rol		r19			; r19 = (t0 >> 7)
.endm

#define R0		r24
#define R1		r25
#define R2		r26

fb_rdcn_low:
	push	r28
	push	r29

	movw	r30, r24	; copy c to z
	movw	r28, r22	; copy a to y

	clr		r20
	clr		r25

	LOAD	58
	ldd		R0, y + 27
	eor		R0, r20
	ldd		r27, y + 28
	eor		r27, r21
	std		y + 28, r27
	ldd		r27, y + 29
	eor		r27, r22
	std		y + 29, r27
	ldd		R1, y + 37
	eor		R1, r23
	ldd		r27, y + 38
	eor		r27, r18
	std		y + 38, r27
	ldd		r27, y + 39
	eor		r27, r19
	std		y + 39, r27

	LOAD	56
	ldd		R2, y + 25
	eor		R2, r20
	ldd		r27, y + 26
	eor		r27, r21
	std		y + 26, r27
	eor		R0, r22
	std		y + 27, R0
	ldd		R0, y + 35
	eor		R0, r23
	ldd		r27, y + 36
	eor		r27, r18
	std		y + 36, r27
	eor		R1, r19
	std		y + 37, R1

	LOAD	54
	ldd		R1, y + 23
	eor		R1, r20
	ldd		r27, y + 24
	eor		r27, r21
	std		y + 24, r27
	eor		R2, r22
	std		y + 25, R2
	ldd		R2, y + 33
	eor		R2, r23
	ldd		r27, y + 34
	eor		r27, r18
	std		y + 34, r27
	eor		R0, r19
	std		y + 35, R0

	LOAD	52
	ldd		R0, y + 21
	eor		R0, r20
	ldd		r27, y + 22
	eor		r27, r21
	std		y + 22, r27
	eor		R1, r22
	std		y + 23, R1
	ldd		R1, y + 31
	eor		R1, r23
	ldd		r27, y + 32
	eor		r27, r18
	std		y + 32, r27
	eor		R2, r19
	std		y + 33, R2

	LOAD	50
	ldd		R2, y + 19
	eor		R2, r20
	ldd		r27, y + 20
	eor		r27, r21
	std		y + 20, r27
	eor		R0, r22
	std		y + 21, R0
	ldd		R0, y + 29
	eor		R0, r23
	ldd		r27, y + 30
	eor		r27, r18
	std		y + 30, r27
	eor		R1, r19
	std		y + 31, R1

	LOAD	48
	ldd		R1, y + 17
	eor		R1, r20
	ldd		r27, y + 18
	eor		r27, r21
	std		y + 18, r27
	eor		R2, r22
	std		y + 19, R2
	ldd		R2, y + 27
	eor		R2, r23
	ldd		r27, y + 28
	eor		r27, r18
	std		z + 28, r27
	eor		R0, r19
	std		y + 29, R0

	LOAD	46
	ldd		R0, y + 15
	eor		R0, r20
	ldd		r27, y + 16
	eor		r27, r21
	std		y + 16, r27
	eor		R1, r22
	std		y + 17, R1
	ldd		R1, y + 25
	eor		R1, r23
	ldd		r27, y + 26
	eor		r27, r18
	std		z + 26, r27
	eor		R2, r19
	std		z + 27, R2

	LOAD	44
	ldd		R2, y + 13
	eor		R2, r20
	ldd		r27, y + 14
	eor		r27, r21
	std		y + 14, r27
	eor		R0, r22
	std		y + 15, R0
	ldd		R0, y + 23
	eor		R0, r23
	ldd		r27, y + 24
	eor		r27, r18
	std		z + 24, r27
	eor		R1, r19
	std		z + 25, R1

	LOAD	42
	ldd		R1, y + 11
	eor		R1, r20
	ldd		r27, y + 12
	eor		r27, r21
	std		y + 12, r27
	eor		R2, r22
	std		y + 13, R2
	ldd		R2, y + 21
	eor		R2, r23
	ldd		r27, y + 22
	eor		r27, r18
	std		z + 22, r27
	eor		R0, r19
	std		z + 23, R0

	LOAD	40
	ldd		R0, y + 9
	eor		R0, r20
	ldd		r27, y + 10
	eor		r27, r21
	std		y + 10, r27
	eor		R1, r22
	std		y + 11, R1
	ldd		R1, y + 19
	eor		R1, r23
	ldd		r27, y + 20
	eor		r27, r18
	std		z + 20, r27
	eor		R2, r19
	std		z + 21, R2

	LOAD	38
	ldd		R2, y + 7
	eor		R2, r20
	ldd		r27, y + 8
	eor		r27, r21
	std		z + 8, r27
	eor		R0, r22
	std		y + 9, R0
	ldd		R0, y + 17
	eor		R0, r23
	ldd		r27, y + 18
	eor		r27, r18
	std		z + 18, r27
	eor		R1, r19
	std		z + 19, R1

	LOAD	36
	ldd		R1, y + 5
	eor		R1, r20
	ldd		r27, y + 6
	eor		r27, r21
	std		z + 6, r27
	eor		R2, r22
	std		z + 7, R2
	ldd		R2, y + 15
	eor		R2, r23
	ldd		r27, y + 16
	eor		r27, r18
	std		z + 16, r27
	eor		R0, r19
	std		z + 17, R0

	LOAD	34
	ldd		R0, y + 3
	eor		R0, r20
	ldd		r27, y + 4
	eor		r27, r21
	std		z + 4, r27
	eor		R1, r22
	std		z + 5, R1
	ldd		R1, y + 13
	eor		R1, r23
	ldd		r27, y + 14
	eor		r27, r18
	std		z + 14, r27
	eor		R2, r19
	std		z + 15, R2

	LOAD	32
	ldd		R2, y + 1
	eor		R2, r20
	ldd		r27, y + 2
	eor		r27, r21
	std		z + 2, r27
	eor		R0, r22
	std		z + 3, R0
	ldd		R0, y + 11
	eor		R0, r23
	ldd		r27, y + 12
	eor		r27, r18
	std		z + 12, r27
	eor		R1, r19
	std		z + 13, R1

	ldd		r18, y + 30
	ldd		R1, y + 0
	bst		r18, 0
	bld		r20, 7
	eor		R1, r20

	mov		r21, r18
	lsr		r21
	eor		R2, r21
	std		z + 1, R2

	ldd		R2, y + 10
	mov		r21, r18
	lsl		r21
	eor		R2, r21

	clr		r21
	bst		r18, 7
	bld		r21, 0
	eor		R0, r21
	std		z + 11, R0

	ldd		r18, y + 29
	lsr		r18

	eor		R1, r18
	std		z + 0, R1

	ldd		r19, y + 9
	mov		r21, r18
	lsl		r21
	lsl		r21
	eor		r19, r21
	std		z + 9, r19

	mov		r21, r18
	swap	r21
	andi	r21, 0x0F
	lsr		r21
	lsr		r21
	eor		R2, r21
	std		z + 10, R2

	ldd		r19, y + 29
	andi	r19, 0x01
	std		z + 29, r19

	pop		r29
	pop		r28
	ret

#endif

#if FB_POLYN == 271

.global fb_rdcn_low

fb_rdcn_low:
	push	r28
	push	r29

	movw	r30,r24				; Copy c to z
	movw	r28,r22				; Copy a to y
	movw	r26,r28				; copy a + 68 to x
	adiw	r26, 48
	adiw	r26, 20

	ld		r18, -x
	ldd		r19, y + 33
	ldd		r20, y + 34
	mov		r22, r18
	lsl		r22
	eor		r19, r22
	bst		r18, 7
	clr		r22
	bld		r22, 0
	eor		r20, r22
	std		y + 33, r19
	std		y + 34, r20

	ldd		r19, y+(67 - 20)
	eor		r19, r18
	std		y+(67 - 20), r19
	ldd		r19, y+(67 - 12)
	eor		r19, r18
	std		y+(67 - 12), r19
	ldd		r19, y+(67 - 8)
	eor		r19, r18
	std		y+(67 - 8), r19

	clr r24
	clr	r19

	ld		r18, -x
	ldd		r20, y+(66 - 20)
	eor		r20, r18
	std		y+(66 - 20), r20
	ldd		r21, y+(66 - 12)
	eor		r21, r18
	ldd		r20, y+(66 - 8)
	eor		r20, r18
	std		y+(66 - 8), r20
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(66 - 33)
	eor		r20, r24
	std		y+(66 - 33), r20

	ld		r18, -x
	ldd		r20, y+(66 - 21)
	eor		r20, r18
	std		y+(66 - 21), r20
	ldd		r22, y+(66 - 13)
	eor		r22, r18
	ldd		r20, y+(66 - 9)
	eor		r20, r18
	std		y+(66 - 9), r20
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(66 - 34)
	eor		r20, r25
	std		y+(66 - 34), r20

	ld		r18, -x
	ldd		r20, y+(64 - 20)
	eor		r20, r18
	std		y+(64 - 20), r20
	ldd		r23, y+(64 - 12)
	eor		r23, r18
	ldd		r20, y+(64 - 8)
	eor		r20, r18
	std		y+(64 - 8), r20
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(64 - 33)
	eor		r20, r24
	std		y+(64 - 33), r20

	ld		r18, -x
	ldd		r20, y+(64 - 21)
	eor		r20, r18
	std		y+(64 - 21), r20
	ldd		r1, y+(64 - 13)
	eor		r1, r18
	ldd		r20, y+(64 - 9)
	eor		r20, r18
	std		y+(64 - 9), r20
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(64 - 34)
	eor		r20, r25
	std		y+(64 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(62 - 8), r21
	ldd		r20, y+(62 - 20)
	eor		r20, r18
	std		y+(62 - 20), r20
	ldd		r21, y+(62 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(62 - 33)
	eor		r20, r24
	std		y+(62 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(62 - 9), r22
	ldd		r20, y+(62 - 21)
	eor		r20, r18
	std		y+(62 - 21), r20
	ldd		r22, y+(62 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(62 - 34)
	eor		r20, r25
	std		y+(62 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(60 - 8), r23
	ldd		r20, y+(60 - 20)
	eor		r20, r18
	std		y+(60 - 20), r20
	ldd		r23, y+(60 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(60 - 33)
	eor		r20, r24
	std		y+(60 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(60 - 9), r1
	ldd		r20, y+(60 - 21)
	eor		r20, r18
	std		y+(60 - 21), r20
	ldd		r1, y+(60 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(60 - 34)
	eor		r20, r25
	std		y+(60 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(58 - 8), r21
	ldd		r20, y+(58 - 20)
	eor		r20, r18
	std		y+(58 - 20), r20
	ldd		r21, y+(58 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(58 - 33)
	eor		r20, r24
	std		y+(58 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(58 - 9), r22
	ldd		r20, y+(58 - 21)
	eor		r20, r18
	std		y+(58 - 21), r20
	ldd		r22, y+(58 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(58 - 34)
	eor		r20, r25
	std		y+(58 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(56 - 8), r23
	ldd		r20, y+(56 - 20)
	eor		r20, r18
	std		y+(56 - 20), r20
	ldd		r23, y+(56 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(56 - 33)
	eor		r20, r24
	std		y+(56 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(56 - 9), r1
	ldd		r20, y+(56 - 21)
	eor		r20, r18
	std		y+(56 - 21), r20
	ldd		r1, y+(56 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(56 - 34)
	eor		r20, r25
	std		y+(56 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(54 - 8), r21
	ldd		r20, y+(54 - 20)
	eor		r20, r18
	std		y+(54 - 20), r20
	ldd		r21, y+(54 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(54 - 33)
	eor		r20, r24
	std		y+(54 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(54 - 9), r22
	ldd		r20, y+(54 - 21)
	eor		r20, r18
	std		y+(54 - 21), r20
	ldd		r22, y+(54 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(54 - 34)
	eor		r20, r25
	std		y+(54 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(52 - 8), r23
	ldd		r20, y+(52 - 20)
	eor		r20, r18
	std		y+(52 - 20), r20
	ldd		r23, y+(52 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(52 - 33)
	eor		r20, r24
	std		y+(52 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(52 - 9), r1
	ldd		r20, y+(52 - 21)
	eor		r20, r18
	std		y+(52 - 21), r20
	ldd		r1, y+(52 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(52 - 34)
	eor		r20, r25
	std		y+(52 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(50 - 8), r21
	ldd		r20, y+(50 - 20)
	eor		r20, r18
	std		y+(50 - 20), r20
	ldd		r21, y+(50 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(50 - 33)
	eor		r20, r24
	std		y+(50 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(50 - 9), r22
	ldd		r20, y+(50 - 21)
	eor		r20, r18
	std		y+(50 - 21), r20
	ldd		r22, y+(50 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(50 - 34)
	eor		r20, r25
	std		y+(50 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(48 - 8), r23
	ldd		r20, y+(48 - 20)
	eor		r20, r18
	std		y+(48 - 20), r20
	ldd		r23, y+(48 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(48 - 33)
	eor		r20, r24
	std		y+(48 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(48 - 9), r1
	ldd		r20, y+(48 - 21)
	eor		r20, r18
	std		y+(48 - 21), r20
	ldd		r1, y+(48 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(48 - 34)
	eor		r20, r25
	std		y+(48 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(46 - 8), r21
	ldd		r20, y+(46 - 20)
	eor		r20, r18
	std		y+(46 - 20), r20
	ldd		r21, y+(46 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(46 - 33)
	eor		r20, r24
	std		z+(46 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		y+(46 - 9), r22
	ldd		r20, y+(46 - 21)
	eor		r20, r18
	std		y+(46 - 21), r20
	ldd		r22, y+(46 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(46 - 34)
	eor		r20, r25
	std		z+(46 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		y+(44 - 8), r23
	ldd		r20, y+(44 - 20)
	eor		r20, r18
	std		y+(44 - 20), r20
	ldd		r23, y+(44 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(44 - 33)
	eor		r20, r24
	std		z+(44 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		y+(44 - 9), r1
	ldd		r20, y+(44 - 21)
	eor		r20, r18
	std		y+(44 - 21), r20
	ldd		r1, y+(44 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(44 - 34)
	eor		r20, r25
	std		z+(44 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		y+(42 - 8), r21
	ldd		r20, y+(42 - 20)
	eor		r20, r18
	std		y+(42 - 20), r20
	ldd		r21, y+(42 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(42 - 33)
	eor		r20, r24
	std		z+(42 - 33), r20

	ld		r18, -x
	eor		r22, r18
	std		z+(42 - 9), r22
	ldd		r20, y+(42 - 21)
	eor		r20, r18
	std		z+(42 - 21), r20
	ldd		r22, y+(42 - 13)
	eor		r22, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(42 - 34)
	eor		r20, r25
	std		z+(42 - 34), r20

	ld		r18, -x
	eor		r23, r18
	std		z+(40 - 8), r23
	ldd		r20, y+(40 - 20)
	eor		r20, r18
	std		z+(40 - 20), r20
	ldd		r23, y+(40 - 12)
	eor		r23, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(40 - 33)
	eor		r20, r24
	std		z+(40 - 33), r20

	ld		r18, -x
	eor		r1, r18
	std		z+(40 - 9), r1
	ldd		r20, y+(40 - 21)
	eor		r20, r18
	std		z+(40 - 21), r20
	ldd		r1, y+(40 - 13)
	eor		r1, r18
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(40 - 34)
	eor		r20, r25
	std		z+(40 - 34), r20

	ld		r18, -x
	eor		r21, r18
	std		z+(38 - 8), r21
	ldd		r20, y+(38 - 20)
	eor		r20, r18
	std		z+(38 - 20), r20
	ldd		r21, y+(38 - 12)
	eor		r21, r18
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(38 - 33)
	eor		r20, r24
	std		z+(38 - 33), r20

	ld		r18, -x
	ldd		r20, y+(38 - 21)
	eor		r20, r18
	std		z+(38 - 21), r20
	ldd		r20, y+(38 - 13)
	eor		r20, r18
	std		z+(38 - 13), r20
	eor		r22, r18
	std		z+(38 - 9), r22
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(38 - 34)
	eor		r20, r25
	std		z+(38 - 34), r20

	ld		r18, -x
	ldd		r20, y+(36 - 20)
	eor		r20, r18
	std		z+(36 - 20), r20
	ldd		r20, y+(36 - 12)
	eor		r20, r18
	std		z+(36 - 12), r20
	eor		r23, r18
	std		z+(36 - 8), r23
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	mov		r25, r18
	lsl		r25
	ldd		r20, y+(36 - 33)
	eor		r20, r24
	std		z+(36 - 33), r20

	ld		r18, -x
	ldd		r20, y+(36 - 21)
	eor		r20, r18
	std		z+(36 - 21), r20
	ldd		r20, y+(36 - 13)
	eor		r20, r18
	std		z+(36 - 13), r20
	eor		r1, r18
	std		z+(36 - 9), r1
	bst		r18, 7
	bld		r19, 0
	eor		r25, r19
	mov		r24, r18
	lsl		r24
	ldd		r20, y+(36 - 34)
	eor		r20, r25
	std		z+(36 - 34), r20

	ldd		r18, y+34
	ldd		r20, y+(34 - 20)
	eor		r20, r18
	std		z+(34 - 20), r20
	ldd		r20, y+(34 - 12)
	eor		r20, r18
	std		z+(34 - 12), r20
	eor		r21, r18
	std		z+(34 - 8), r21
	bst		r18, 7
	bld		r19, 0
	eor		r24, r19
	lsl		r18
	mov		r25, r18
	ldd		r18, y + 1
	eor		r18, r24
	std		z + 1, r18
	ldd		r18, y + 0
	eor		r18, r25
	std		z + 0, r18

	ldd		r25, z + 33
	mov		r24, r25
	andi	r24, 0x80
	tst		r24
	breq	end
	ldi		r19, 0x01
	ldi		r20, 0x80
	ldd		r18, z + 0
	eor		r18, r19
	std		z + 0, r18
	ldd		r18, z + 13
	eor		r18, r20
	std		z + 13, r18
	ldd		r18, z + 21
	eor		r18, r20
	std		z + 21, r18
	ldd		r18, z + 25
	eor		r18, r20
	std		z + 25, r18
	andi	r25, 0x7F
	std		z + 33, r25

end:
	clr		r1

	pop		r29
	pop		r28
	ret
#endif

