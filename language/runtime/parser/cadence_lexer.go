// Code generated from parser/Cadence.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser

import (
	"fmt"
	"unicode"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// Suppress unused import error
var _ = fmt.Printf
var _ = unicode.IsLetter

var serializedLexerAtn = []uint16{
	3, 24715, 42794, 33075, 47597, 16764, 15335, 30598, 22884, 2, 74, 538,
	8, 1, 4, 2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7,
	9, 7, 4, 8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12,
	4, 13, 9, 13, 4, 14, 9, 14, 4, 15, 9, 15, 4, 16, 9, 16, 4, 17, 9, 17, 4,
	18, 9, 18, 4, 19, 9, 19, 4, 20, 9, 20, 4, 21, 9, 21, 4, 22, 9, 22, 4, 23,
	9, 23, 4, 24, 9, 24, 4, 25, 9, 25, 4, 26, 9, 26, 4, 27, 9, 27, 4, 28, 9,
	28, 4, 29, 9, 29, 4, 30, 9, 30, 4, 31, 9, 31, 4, 32, 9, 32, 4, 33, 9, 33,
	4, 34, 9, 34, 4, 35, 9, 35, 4, 36, 9, 36, 4, 37, 9, 37, 4, 38, 9, 38, 4,
	39, 9, 39, 4, 40, 9, 40, 4, 41, 9, 41, 4, 42, 9, 42, 4, 43, 9, 43, 4, 44,
	9, 44, 4, 45, 9, 45, 4, 46, 9, 46, 4, 47, 9, 47, 4, 48, 9, 48, 4, 49, 9,
	49, 4, 50, 9, 50, 4, 51, 9, 51, 4, 52, 9, 52, 4, 53, 9, 53, 4, 54, 9, 54,
	4, 55, 9, 55, 4, 56, 9, 56, 4, 57, 9, 57, 4, 58, 9, 58, 4, 59, 9, 59, 4,
	60, 9, 60, 4, 61, 9, 61, 4, 62, 9, 62, 4, 63, 9, 63, 4, 64, 9, 64, 4, 65,
	9, 65, 4, 66, 9, 66, 4, 67, 9, 67, 4, 68, 9, 68, 4, 69, 9, 69, 4, 70, 9,
	70, 4, 71, 9, 71, 4, 72, 9, 72, 4, 73, 9, 73, 4, 74, 9, 74, 4, 75, 9, 75,
	4, 76, 9, 76, 4, 77, 9, 77, 4, 78, 9, 78, 3, 2, 3, 2, 3, 3, 3, 3, 3, 4,
	3, 4, 3, 5, 3, 5, 3, 6, 3, 6, 3, 7, 3, 7, 3, 8, 3, 8, 3, 9, 3, 9, 3, 9,
	3, 9, 3, 10, 3, 10, 3, 11, 3, 11, 3, 11, 3, 12, 3, 12, 3, 12, 3, 13, 3,
	13, 3, 14, 3, 14, 3, 14, 3, 15, 3, 15, 3, 15, 3, 16, 3, 16, 3, 17, 3, 17,
	3, 18, 3, 18, 3, 18, 3, 19, 3, 19, 3, 19, 3, 20, 3, 20, 3, 21, 3, 21, 3,
	22, 3, 22, 3, 23, 3, 23, 3, 24, 3, 24, 3, 25, 3, 25, 3, 26, 3, 26, 3, 27,
	3, 27, 3, 27, 3, 28, 3, 28, 3, 29, 3, 29, 3, 29, 3, 29, 3, 30, 3, 30, 3,
	30, 3, 31, 3, 31, 3, 31, 3, 31, 3, 32, 3, 32, 3, 33, 3, 33, 3, 34, 3, 34,
	3, 34, 3, 34, 3, 34, 3, 34, 3, 34, 3, 34, 3, 34, 3, 34, 3, 34, 3, 34, 3,
	35, 3, 35, 3, 35, 3, 35, 3, 35, 3, 35, 3, 35, 3, 36, 3, 36, 3, 36, 3, 36,
	3, 36, 3, 36, 3, 36, 3, 36, 3, 36, 3, 37, 3, 37, 3, 37, 3, 37, 3, 37, 3,
	37, 3, 37, 3, 37, 3, 37, 3, 38, 3, 38, 3, 38, 3, 38, 3, 38, 3, 38, 3, 38,
	3, 38, 3, 38, 3, 38, 3, 39, 3, 39, 3, 39, 3, 39, 3, 40, 3, 40, 3, 40, 3,
	40, 3, 40, 3, 40, 3, 41, 3, 41, 3, 41, 3, 41, 3, 41, 3, 42, 3, 42, 3, 42,
	3, 42, 3, 43, 3, 43, 3, 43, 3, 43, 3, 43, 3, 44, 3, 44, 3, 44, 3, 44, 3,
	44, 3, 45, 3, 45, 3, 45, 3, 45, 3, 45, 3, 46, 3, 46, 3, 46, 3, 46, 3, 47,
	3, 47, 3, 47, 3, 47, 3, 47, 3, 47, 3, 47, 3, 47, 3, 47, 3, 48, 3, 48, 3,
	48, 3, 48, 3, 48, 3, 48, 3, 48, 3, 49, 3, 49, 3, 49, 3, 49, 3, 49, 3, 49,
	3, 50, 3, 50, 3, 50, 3, 50, 3, 50, 3, 50, 3, 50, 3, 50, 3, 50, 3, 51, 3,
	51, 3, 51, 3, 51, 3, 52, 3, 52, 3, 52, 3, 52, 3, 53, 3, 53, 3, 53, 3, 54,
	3, 54, 3, 54, 3, 54, 3, 54, 3, 55, 3, 55, 3, 55, 3, 55, 3, 55, 3, 55, 3,
	56, 3, 56, 3, 56, 3, 56, 3, 56, 3, 57, 3, 57, 3, 57, 3, 57, 3, 57, 3, 57,
	3, 58, 3, 58, 3, 58, 3, 58, 3, 59, 3, 59, 3, 59, 3, 59, 3, 59, 3, 59, 3,
	59, 3, 60, 3, 60, 3, 60, 3, 60, 3, 60, 3, 61, 3, 61, 3, 61, 3, 61, 3, 61,
	3, 61, 3, 61, 3, 62, 3, 62, 3, 62, 3, 62, 3, 62, 3, 62, 3, 62, 3, 62, 3,
	63, 3, 63, 7, 63, 418, 10, 63, 12, 63, 14, 63, 421, 11, 63, 3, 64, 5, 64,
	424, 10, 64, 3, 65, 3, 65, 5, 65, 428, 10, 65, 3, 66, 3, 66, 7, 66, 432,
	10, 66, 12, 66, 14, 66, 435, 11, 66, 3, 67, 3, 67, 3, 67, 3, 67, 6, 67,
	441, 10, 67, 13, 67, 14, 67, 442, 3, 68, 3, 68, 3, 68, 3, 68, 6, 68, 449,
	10, 68, 13, 68, 14, 68, 450, 3, 69, 3, 69, 3, 69, 3, 69, 6, 69, 457, 10,
	69, 13, 69, 14, 69, 458, 3, 70, 3, 70, 3, 70, 7, 70, 464, 10, 70, 12, 70,
	14, 70, 467, 11, 70, 3, 71, 3, 71, 7, 71, 471, 10, 71, 12, 71, 14, 71,
	474, 11, 71, 3, 71, 3, 71, 3, 72, 3, 72, 5, 72, 480, 10, 72, 3, 73, 3,
	73, 3, 73, 3, 73, 3, 73, 3, 73, 3, 73, 6, 73, 489, 10, 73, 13, 73, 14,
	73, 490, 3, 73, 3, 73, 5, 73, 495, 10, 73, 3, 74, 3, 74, 3, 75, 6, 75,
	500, 10, 75, 13, 75, 14, 75, 501, 3, 75, 3, 75, 3, 76, 6, 76, 507, 10,
	76, 13, 76, 14, 76, 508, 3, 76, 3, 76, 3, 77, 3, 77, 3, 77, 3, 77, 3, 77,
	7, 77, 518, 10, 77, 12, 77, 14, 77, 521, 11, 77, 3, 77, 3, 77, 3, 77, 3,
	77, 3, 77, 3, 78, 3, 78, 3, 78, 3, 78, 7, 78, 532, 10, 78, 12, 78, 14,
	78, 535, 11, 78, 3, 78, 3, 78, 3, 519, 2, 79, 3, 3, 5, 4, 7, 5, 9, 6, 11,
	7, 13, 8, 15, 9, 17, 10, 19, 11, 21, 12, 23, 13, 25, 14, 27, 15, 29, 16,
	31, 17, 33, 18, 35, 19, 37, 20, 39, 21, 41, 22, 43, 23, 45, 24, 47, 25,
	49, 26, 51, 27, 53, 28, 55, 29, 57, 30, 59, 31, 61, 32, 63, 33, 65, 34,
	67, 35, 69, 36, 71, 37, 73, 38, 75, 39, 77, 40, 79, 41, 81, 42, 83, 43,
	85, 44, 87, 45, 89, 46, 91, 47, 93, 48, 95, 49, 97, 50, 99, 51, 101, 52,
	103, 53, 105, 54, 107, 55, 109, 56, 111, 57, 113, 58, 115, 59, 117, 60,
	119, 61, 121, 62, 123, 63, 125, 64, 127, 2, 129, 2, 131, 65, 133, 66, 135,
	67, 137, 68, 139, 69, 141, 70, 143, 2, 145, 2, 147, 2, 149, 71, 151, 72,
	153, 73, 155, 74, 3, 2, 15, 5, 2, 67, 92, 97, 97, 99, 124, 3, 2, 50, 59,
	4, 2, 50, 59, 97, 97, 4, 2, 50, 51, 97, 97, 4, 2, 50, 57, 97, 97, 6, 2,
	50, 59, 67, 72, 97, 97, 99, 104, 4, 2, 67, 92, 99, 124, 6, 2, 50, 59, 67,
	92, 97, 97, 99, 124, 6, 2, 12, 12, 15, 15, 36, 36, 94, 94, 9, 2, 36, 36,
	41, 41, 50, 50, 94, 94, 112, 112, 116, 116, 118, 118, 5, 2, 50, 59, 67,
	72, 99, 104, 6, 2, 2, 2, 11, 11, 13, 14, 34, 34, 4, 2, 12, 12, 15, 15,
	2, 548, 2, 3, 3, 2, 2, 2, 2, 5, 3, 2, 2, 2, 2, 7, 3, 2, 2, 2, 2, 9, 3,
	2, 2, 2, 2, 11, 3, 2, 2, 2, 2, 13, 3, 2, 2, 2, 2, 15, 3, 2, 2, 2, 2, 17,
	3, 2, 2, 2, 2, 19, 3, 2, 2, 2, 2, 21, 3, 2, 2, 2, 2, 23, 3, 2, 2, 2, 2,
	25, 3, 2, 2, 2, 2, 27, 3, 2, 2, 2, 2, 29, 3, 2, 2, 2, 2, 31, 3, 2, 2, 2,
	2, 33, 3, 2, 2, 2, 2, 35, 3, 2, 2, 2, 2, 37, 3, 2, 2, 2, 2, 39, 3, 2, 2,
	2, 2, 41, 3, 2, 2, 2, 2, 43, 3, 2, 2, 2, 2, 45, 3, 2, 2, 2, 2, 47, 3, 2,
	2, 2, 2, 49, 3, 2, 2, 2, 2, 51, 3, 2, 2, 2, 2, 53, 3, 2, 2, 2, 2, 55, 3,
	2, 2, 2, 2, 57, 3, 2, 2, 2, 2, 59, 3, 2, 2, 2, 2, 61, 3, 2, 2, 2, 2, 63,
	3, 2, 2, 2, 2, 65, 3, 2, 2, 2, 2, 67, 3, 2, 2, 2, 2, 69, 3, 2, 2, 2, 2,
	71, 3, 2, 2, 2, 2, 73, 3, 2, 2, 2, 2, 75, 3, 2, 2, 2, 2, 77, 3, 2, 2, 2,
	2, 79, 3, 2, 2, 2, 2, 81, 3, 2, 2, 2, 2, 83, 3, 2, 2, 2, 2, 85, 3, 2, 2,
	2, 2, 87, 3, 2, 2, 2, 2, 89, 3, 2, 2, 2, 2, 91, 3, 2, 2, 2, 2, 93, 3, 2,
	2, 2, 2, 95, 3, 2, 2, 2, 2, 97, 3, 2, 2, 2, 2, 99, 3, 2, 2, 2, 2, 101,
	3, 2, 2, 2, 2, 103, 3, 2, 2, 2, 2, 105, 3, 2, 2, 2, 2, 107, 3, 2, 2, 2,
	2, 109, 3, 2, 2, 2, 2, 111, 3, 2, 2, 2, 2, 113, 3, 2, 2, 2, 2, 115, 3,
	2, 2, 2, 2, 117, 3, 2, 2, 2, 2, 119, 3, 2, 2, 2, 2, 121, 3, 2, 2, 2, 2,
	123, 3, 2, 2, 2, 2, 125, 3, 2, 2, 2, 2, 131, 3, 2, 2, 2, 2, 133, 3, 2,
	2, 2, 2, 135, 3, 2, 2, 2, 2, 137, 3, 2, 2, 2, 2, 139, 3, 2, 2, 2, 2, 141,
	3, 2, 2, 2, 2, 149, 3, 2, 2, 2, 2, 151, 3, 2, 2, 2, 2, 153, 3, 2, 2, 2,
	2, 155, 3, 2, 2, 2, 3, 157, 3, 2, 2, 2, 5, 159, 3, 2, 2, 2, 7, 161, 3,
	2, 2, 2, 9, 163, 3, 2, 2, 2, 11, 165, 3, 2, 2, 2, 13, 167, 3, 2, 2, 2,
	15, 169, 3, 2, 2, 2, 17, 171, 3, 2, 2, 2, 19, 175, 3, 2, 2, 2, 21, 177,
	3, 2, 2, 2, 23, 180, 3, 2, 2, 2, 25, 183, 3, 2, 2, 2, 27, 185, 3, 2, 2,
	2, 29, 188, 3, 2, 2, 2, 31, 191, 3, 2, 2, 2, 33, 193, 3, 2, 2, 2, 35, 195,
	3, 2, 2, 2, 37, 198, 3, 2, 2, 2, 39, 201, 3, 2, 2, 2, 41, 203, 3, 2, 2,
	2, 43, 205, 3, 2, 2, 2, 45, 207, 3, 2, 2, 2, 47, 209, 3, 2, 2, 2, 49, 211,
	3, 2, 2, 2, 51, 213, 3, 2, 2, 2, 53, 215, 3, 2, 2, 2, 55, 218, 3, 2, 2,
	2, 57, 220, 3, 2, 2, 2, 59, 224, 3, 2, 2, 2, 61, 227, 3, 2, 2, 2, 63, 231,
	3, 2, 2, 2, 65, 233, 3, 2, 2, 2, 67, 235, 3, 2, 2, 2, 69, 247, 3, 2, 2,
	2, 71, 254, 3, 2, 2, 2, 73, 263, 3, 2, 2, 2, 75, 272, 3, 2, 2, 2, 77, 282,
	3, 2, 2, 2, 79, 286, 3, 2, 2, 2, 81, 292, 3, 2, 2, 2, 83, 297, 3, 2, 2,
	2, 85, 301, 3, 2, 2, 2, 87, 306, 3, 2, 2, 2, 89, 311, 3, 2, 2, 2, 91, 316,
	3, 2, 2, 2, 93, 320, 3, 2, 2, 2, 95, 329, 3, 2, 2, 2, 97, 336, 3, 2, 2,
	2, 99, 342, 3, 2, 2, 2, 101, 351, 3, 2, 2, 2, 103, 355, 3, 2, 2, 2, 105,
	359, 3, 2, 2, 2, 107, 362, 3, 2, 2, 2, 109, 367, 3, 2, 2, 2, 111, 373,
	3, 2, 2, 2, 113, 378, 3, 2, 2, 2, 115, 384, 3, 2, 2, 2, 117, 388, 3, 2,
	2, 2, 119, 395, 3, 2, 2, 2, 121, 400, 3, 2, 2, 2, 123, 407, 3, 2, 2, 2,
	125, 415, 3, 2, 2, 2, 127, 423, 3, 2, 2, 2, 129, 427, 3, 2, 2, 2, 131,
	429, 3, 2, 2, 2, 133, 436, 3, 2, 2, 2, 135, 444, 3, 2, 2, 2, 137, 452,
	3, 2, 2, 2, 139, 460, 3, 2, 2, 2, 141, 468, 3, 2, 2, 2, 143, 479, 3, 2,
	2, 2, 145, 494, 3, 2, 2, 2, 147, 496, 3, 2, 2, 2, 149, 499, 3, 2, 2, 2,
	151, 506, 3, 2, 2, 2, 153, 512, 3, 2, 2, 2, 155, 527, 3, 2, 2, 2, 157,
	158, 7, 61, 2, 2, 158, 4, 3, 2, 2, 2, 159, 160, 7, 125, 2, 2, 160, 6, 3,
	2, 2, 2, 161, 162, 7, 127, 2, 2, 162, 8, 3, 2, 2, 2, 163, 164, 7, 46, 2,
	2, 164, 10, 3, 2, 2, 2, 165, 166, 7, 60, 2, 2, 166, 12, 3, 2, 2, 2, 167,
	168, 7, 93, 2, 2, 168, 14, 3, 2, 2, 2, 169, 170, 7, 95, 2, 2, 170, 16,
	3, 2, 2, 2, 171, 172, 7, 62, 2, 2, 172, 173, 7, 47, 2, 2, 173, 174, 7,
	64, 2, 2, 174, 18, 3, 2, 2, 2, 175, 176, 7, 63, 2, 2, 176, 20, 3, 2, 2,
	2, 177, 178, 7, 126, 2, 2, 178, 179, 7, 126, 2, 2, 179, 22, 3, 2, 2, 2,
	180, 181, 7, 40, 2, 2, 181, 182, 7, 40, 2, 2, 182, 24, 3, 2, 2, 2, 183,
	184, 7, 48, 2, 2, 184, 26, 3, 2, 2, 2, 185, 186, 7, 63, 2, 2, 186, 187,
	7, 63, 2, 2, 187, 28, 3, 2, 2, 2, 188, 189, 7, 35, 2, 2, 189, 190, 7, 63,
	2, 2, 190, 30, 3, 2, 2, 2, 191, 192, 7, 62, 2, 2, 192, 32, 3, 2, 2, 2,
	193, 194, 7, 64, 2, 2, 194, 34, 3, 2, 2, 2, 195, 196, 7, 62, 2, 2, 196,
	197, 7, 63, 2, 2, 197, 36, 3, 2, 2, 2, 198, 199, 7, 64, 2, 2, 199, 200,
	7, 63, 2, 2, 200, 38, 3, 2, 2, 2, 201, 202, 7, 45, 2, 2, 202, 40, 3, 2,
	2, 2, 203, 204, 7, 47, 2, 2, 204, 42, 3, 2, 2, 2, 205, 206, 7, 44, 2, 2,
	206, 44, 3, 2, 2, 2, 207, 208, 7, 49, 2, 2, 208, 46, 3, 2, 2, 2, 209, 210,
	7, 39, 2, 2, 210, 48, 3, 2, 2, 2, 211, 212, 7, 40, 2, 2, 212, 50, 3, 2,
	2, 2, 213, 214, 7, 35, 2, 2, 214, 52, 3, 2, 2, 2, 215, 216, 7, 62, 2, 2,
	216, 217, 7, 47, 2, 2, 217, 54, 3, 2, 2, 2, 218, 219, 7, 65, 2, 2, 219,
	56, 3, 2, 2, 2, 220, 221, 5, 149, 75, 2, 221, 222, 7, 65, 2, 2, 222, 223,
	7, 65, 2, 2, 223, 58, 3, 2, 2, 2, 224, 225, 7, 99, 2, 2, 225, 226, 7, 117,
	2, 2, 226, 60, 3, 2, 2, 2, 227, 228, 7, 99, 2, 2, 228, 229, 7, 117, 2,
	2, 229, 230, 7, 65, 2, 2, 230, 62, 3, 2, 2, 2, 231, 232, 7, 42, 2, 2, 232,
	64, 3, 2, 2, 2, 233, 234, 7, 43, 2, 2, 234, 66, 3, 2, 2, 2, 235, 236, 7,
	118, 2, 2, 236, 237, 7, 116, 2, 2, 237, 238, 7, 99, 2, 2, 238, 239, 7,
	112, 2, 2, 239, 240, 7, 117, 2, 2, 240, 241, 7, 99, 2, 2, 241, 242, 7,
	101, 2, 2, 242, 243, 7, 118, 2, 2, 243, 244, 7, 107, 2, 2, 244, 245, 7,
	113, 2, 2, 245, 246, 7, 112, 2, 2, 246, 68, 3, 2, 2, 2, 247, 248, 7, 117,
	2, 2, 248, 249, 7, 118, 2, 2, 249, 250, 7, 116, 2, 2, 250, 251, 7, 119,
	2, 2, 251, 252, 7, 101, 2, 2, 252, 253, 7, 118, 2, 2, 253, 70, 3, 2, 2,
	2, 254, 255, 7, 116, 2, 2, 255, 256, 7, 103, 2, 2, 256, 257, 7, 117, 2,
	2, 257, 258, 7, 113, 2, 2, 258, 259, 7, 119, 2, 2, 259, 260, 7, 116, 2,
	2, 260, 261, 7, 101, 2, 2, 261, 262, 7, 103, 2, 2, 262, 72, 3, 2, 2, 2,
	263, 264, 7, 101, 2, 2, 264, 265, 7, 113, 2, 2, 265, 266, 7, 112, 2, 2,
	266, 267, 7, 118, 2, 2, 267, 268, 7, 116, 2, 2, 268, 269, 7, 99, 2, 2,
	269, 270, 7, 101, 2, 2, 270, 271, 7, 118, 2, 2, 271, 74, 3, 2, 2, 2, 272,
	273, 7, 107, 2, 2, 273, 274, 7, 112, 2, 2, 274, 275, 7, 118, 2, 2, 275,
	276, 7, 103, 2, 2, 276, 277, 7, 116, 2, 2, 277, 278, 7, 104, 2, 2, 278,
	279, 7, 99, 2, 2, 279, 280, 7, 101, 2, 2, 280, 281, 7, 103, 2, 2, 281,
	76, 3, 2, 2, 2, 282, 283, 7, 104, 2, 2, 283, 284, 7, 119, 2, 2, 284, 285,
	7, 112, 2, 2, 285, 78, 3, 2, 2, 2, 286, 287, 7, 103, 2, 2, 287, 288, 7,
	120, 2, 2, 288, 289, 7, 103, 2, 2, 289, 290, 7, 112, 2, 2, 290, 291, 7,
	118, 2, 2, 291, 80, 3, 2, 2, 2, 292, 293, 7, 103, 2, 2, 293, 294, 7, 111,
	2, 2, 294, 295, 7, 107, 2, 2, 295, 296, 7, 118, 2, 2, 296, 82, 3, 2, 2,
	2, 297, 298, 7, 114, 2, 2, 298, 299, 7, 116, 2, 2, 299, 300, 7, 103, 2,
	2, 300, 84, 3, 2, 2, 2, 301, 302, 7, 114, 2, 2, 302, 303, 7, 113, 2, 2,
	303, 304, 7, 117, 2, 2, 304, 305, 7, 118, 2, 2, 305, 86, 3, 2, 2, 2, 306,
	307, 7, 114, 2, 2, 307, 308, 7, 116, 2, 2, 308, 309, 7, 107, 2, 2, 309,
	310, 7, 120, 2, 2, 310, 88, 3, 2, 2, 2, 311, 312, 7, 99, 2, 2, 312, 313,
	7, 119, 2, 2, 313, 314, 7, 118, 2, 2, 314, 315, 7, 106, 2, 2, 315, 90,
	3, 2, 2, 2, 316, 317, 7, 114, 2, 2, 317, 318, 7, 119, 2, 2, 318, 319, 7,
	100, 2, 2, 319, 92, 3, 2, 2, 2, 320, 321, 7, 114, 2, 2, 321, 322, 7, 119,
	2, 2, 322, 323, 7, 100, 2, 2, 323, 324, 7, 42, 2, 2, 324, 325, 7, 117,
	2, 2, 325, 326, 7, 103, 2, 2, 326, 327, 7, 118, 2, 2, 327, 328, 7, 43,
	2, 2, 328, 94, 3, 2, 2, 2, 329, 330, 7, 116, 2, 2, 330, 331, 7, 103, 2,
	2, 331, 332, 7, 118, 2, 2, 332, 333, 7, 119, 2, 2, 333, 334, 7, 116, 2,
	2, 334, 335, 7, 112, 2, 2, 335, 96, 3, 2, 2, 2, 336, 337, 7, 100, 2, 2,
	337, 338, 7, 116, 2, 2, 338, 339, 7, 103, 2, 2, 339, 340, 7, 99, 2, 2,
	340, 341, 7, 109, 2, 2, 341, 98, 3, 2, 2, 2, 342, 343, 7, 101, 2, 2, 343,
	344, 7, 113, 2, 2, 344, 345, 7, 112, 2, 2, 345, 346, 7, 118, 2, 2, 346,
	347, 7, 107, 2, 2, 347, 348, 7, 112, 2, 2, 348, 349, 7, 119, 2, 2, 349,
	350, 7, 103, 2, 2, 350, 100, 3, 2, 2, 2, 351, 352, 7, 110, 2, 2, 352, 353,
	7, 103, 2, 2, 353, 354, 7, 118, 2, 2, 354, 102, 3, 2, 2, 2, 355, 356, 7,
	120, 2, 2, 356, 357, 7, 99, 2, 2, 357, 358, 7, 116, 2, 2, 358, 104, 3,
	2, 2, 2, 359, 360, 7, 107, 2, 2, 360, 361, 7, 104, 2, 2, 361, 106, 3, 2,
	2, 2, 362, 363, 7, 103, 2, 2, 363, 364, 7, 110, 2, 2, 364, 365, 7, 117,
	2, 2, 365, 366, 7, 103, 2, 2, 366, 108, 3, 2, 2, 2, 367, 368, 7, 121, 2,
	2, 368, 369, 7, 106, 2, 2, 369, 370, 7, 107, 2, 2, 370, 371, 7, 110, 2,
	2, 371, 372, 7, 103, 2, 2, 372, 110, 3, 2, 2, 2, 373, 374, 7, 118, 2, 2,
	374, 375, 7, 116, 2, 2, 375, 376, 7, 119, 2, 2, 376, 377, 7, 103, 2, 2,
	377, 112, 3, 2, 2, 2, 378, 379, 7, 104, 2, 2, 379, 380, 7, 99, 2, 2, 380,
	381, 7, 110, 2, 2, 381, 382, 7, 117, 2, 2, 382, 383, 7, 103, 2, 2, 383,
	114, 3, 2, 2, 2, 384, 385, 7, 112, 2, 2, 385, 386, 7, 107, 2, 2, 386, 387,
	7, 110, 2, 2, 387, 116, 3, 2, 2, 2, 388, 389, 7, 107, 2, 2, 389, 390, 7,
	111, 2, 2, 390, 391, 7, 114, 2, 2, 391, 392, 7, 113, 2, 2, 392, 393, 7,
	116, 2, 2, 393, 394, 7, 118, 2, 2, 394, 118, 3, 2, 2, 2, 395, 396, 7, 104,
	2, 2, 396, 397, 7, 116, 2, 2, 397, 398, 7, 113, 2, 2, 398, 399, 7, 111,
	2, 2, 399, 120, 3, 2, 2, 2, 400, 401, 7, 101, 2, 2, 401, 402, 7, 116, 2,
	2, 402, 403, 7, 103, 2, 2, 403, 404, 7, 99, 2, 2, 404, 405, 7, 118, 2,
	2, 405, 406, 7, 103, 2, 2, 406, 122, 3, 2, 2, 2, 407, 408, 7, 102, 2, 2,
	408, 409, 7, 103, 2, 2, 409, 410, 7, 117, 2, 2, 410, 411, 7, 118, 2, 2,
	411, 412, 7, 116, 2, 2, 412, 413, 7, 113, 2, 2, 413, 414, 7, 123, 2, 2,
	414, 124, 3, 2, 2, 2, 415, 419, 5, 127, 64, 2, 416, 418, 5, 129, 65, 2,
	417, 416, 3, 2, 2, 2, 418, 421, 3, 2, 2, 2, 419, 417, 3, 2, 2, 2, 419,
	420, 3, 2, 2, 2, 420, 126, 3, 2, 2, 2, 421, 419, 3, 2, 2, 2, 422, 424,
	9, 2, 2, 2, 423, 422, 3, 2, 2, 2, 424, 128, 3, 2, 2, 2, 425, 428, 9, 3,
	2, 2, 426, 428, 5, 127, 64, 2, 427, 425, 3, 2, 2, 2, 427, 426, 3, 2, 2,
	2, 428, 130, 3, 2, 2, 2, 429, 433, 9, 3, 2, 2, 430, 432, 9, 4, 2, 2, 431,
	430, 3, 2, 2, 2, 432, 435, 3, 2, 2, 2, 433, 431, 3, 2, 2, 2, 433, 434,
	3, 2, 2, 2, 434, 132, 3, 2, 2, 2, 435, 433, 3, 2, 2, 2, 436, 437, 7, 50,
	2, 2, 437, 438, 7, 100, 2, 2, 438, 440, 3, 2, 2, 2, 439, 441, 9, 5, 2,
	2, 440, 439, 3, 2, 2, 2, 441, 442, 3, 2, 2, 2, 442, 440, 3, 2, 2, 2, 442,
	443, 3, 2, 2, 2, 443, 134, 3, 2, 2, 2, 444, 445, 7, 50, 2, 2, 445, 446,
	7, 113, 2, 2, 446, 448, 3, 2, 2, 2, 447, 449, 9, 6, 2, 2, 448, 447, 3,
	2, 2, 2, 449, 450, 3, 2, 2, 2, 450, 448, 3, 2, 2, 2, 450, 451, 3, 2, 2,
	2, 451, 136, 3, 2, 2, 2, 452, 453, 7, 50, 2, 2, 453, 454, 7, 122, 2, 2,
	454, 456, 3, 2, 2, 2, 455, 457, 9, 7, 2, 2, 456, 455, 3, 2, 2, 2, 457,
	458, 3, 2, 2, 2, 458, 456, 3, 2, 2, 2, 458, 459, 3, 2, 2, 2, 459, 138,
	3, 2, 2, 2, 460, 461, 7, 50, 2, 2, 461, 465, 9, 8, 2, 2, 462, 464, 9, 9,
	2, 2, 463, 462, 3, 2, 2, 2, 464, 467, 3, 2, 2, 2, 465, 463, 3, 2, 2, 2,
	465, 466, 3, 2, 2, 2, 466, 140, 3, 2, 2, 2, 467, 465, 3, 2, 2, 2, 468,
	472, 7, 36, 2, 2, 469, 471, 5, 143, 72, 2, 470, 469, 3, 2, 2, 2, 471, 474,
	3, 2, 2, 2, 472, 470, 3, 2, 2, 2, 472, 473, 3, 2, 2, 2, 473, 475, 3, 2,
	2, 2, 474, 472, 3, 2, 2, 2, 475, 476, 7, 36, 2, 2, 476, 142, 3, 2, 2, 2,
	477, 480, 5, 145, 73, 2, 478, 480, 10, 10, 2, 2, 479, 477, 3, 2, 2, 2,
	479, 478, 3, 2, 2, 2, 480, 144, 3, 2, 2, 2, 481, 482, 7, 94, 2, 2, 482,
	495, 9, 11, 2, 2, 483, 484, 7, 94, 2, 2, 484, 485, 7, 119, 2, 2, 485, 486,
	3, 2, 2, 2, 486, 488, 7, 125, 2, 2, 487, 489, 5, 147, 74, 2, 488, 487,
	3, 2, 2, 2, 489, 490, 3, 2, 2, 2, 490, 488, 3, 2, 2, 2, 490, 491, 3, 2,
	2, 2, 491, 492, 3, 2, 2, 2, 492, 493, 7, 127, 2, 2, 493, 495, 3, 2, 2,
	2, 494, 481, 3, 2, 2, 2, 494, 483, 3, 2, 2, 2, 495, 146, 3, 2, 2, 2, 496,
	497, 9, 12, 2, 2, 497, 148, 3, 2, 2, 2, 498, 500, 9, 13, 2, 2, 499, 498,
	3, 2, 2, 2, 500, 501, 3, 2, 2, 2, 501, 499, 3, 2, 2, 2, 501, 502, 3, 2,
	2, 2, 502, 503, 3, 2, 2, 2, 503, 504, 8, 75, 2, 2, 504, 150, 3, 2, 2, 2,
	505, 507, 9, 14, 2, 2, 506, 505, 3, 2, 2, 2, 507, 508, 3, 2, 2, 2, 508,
	506, 3, 2, 2, 2, 508, 509, 3, 2, 2, 2, 509, 510, 3, 2, 2, 2, 510, 511,
	8, 76, 2, 2, 511, 152, 3, 2, 2, 2, 512, 513, 7, 49, 2, 2, 513, 514, 7,
	44, 2, 2, 514, 519, 3, 2, 2, 2, 515, 518, 5, 153, 77, 2, 516, 518, 11,
	2, 2, 2, 517, 515, 3, 2, 2, 2, 517, 516, 3, 2, 2, 2, 518, 521, 3, 2, 2,
	2, 519, 520, 3, 2, 2, 2, 519, 517, 3, 2, 2, 2, 520, 522, 3, 2, 2, 2, 521,
	519, 3, 2, 2, 2, 522, 523, 7, 44, 2, 2, 523, 524, 7, 49, 2, 2, 524, 525,
	3, 2, 2, 2, 525, 526, 8, 77, 2, 2, 526, 154, 3, 2, 2, 2, 527, 528, 7, 49,
	2, 2, 528, 529, 7, 49, 2, 2, 529, 533, 3, 2, 2, 2, 530, 532, 10, 14, 2,
	2, 531, 530, 3, 2, 2, 2, 532, 535, 3, 2, 2, 2, 533, 531, 3, 2, 2, 2, 533,
	534, 3, 2, 2, 2, 534, 536, 3, 2, 2, 2, 535, 533, 3, 2, 2, 2, 536, 537,
	8, 78, 2, 2, 537, 156, 3, 2, 2, 2, 20, 2, 419, 423, 427, 433, 442, 450,
	458, 465, 472, 479, 490, 494, 501, 508, 517, 519, 533, 3, 2, 3, 2,
}

var lexerDeserializer = antlr.NewATNDeserializer(nil)
var lexerAtn = lexerDeserializer.DeserializeFromUInt16(serializedLexerAtn)

var lexerChannelNames = []string{
	"DEFAULT_TOKEN_CHANNEL", "HIDDEN",
}

var lexerModeNames = []string{
	"DEFAULT_MODE",
}

var lexerLiteralNames = []string{
	"", "';'", "'{'", "'}'", "','", "':'", "'['", "']'", "'<->'", "'='", "'||'",
	"'&&'", "'.'", "'=='", "'!='", "'<'", "'>'", "'<='", "'>='", "'+'", "'-'",
	"'*'", "'/'", "'%'", "'&'", "'!'", "'<-'", "'?'", "", "'as'", "'as?'",
	"'('", "')'", "'transaction'", "'struct'", "'resource'", "'contract'",
	"'interface'", "'fun'", "'event'", "'emit'", "'pre'", "'post'", "'priv'",
	"'auth'", "'pub'", "'pub(set)'", "'return'", "'break'", "'continue'", "'let'",
	"'var'", "'if'", "'else'", "'while'", "'true'", "'false'", "'nil'", "'import'",
	"'from'", "'create'", "'destroy'",
}

var lexerSymbolicNames = []string{
	"", "", "", "", "", "", "", "", "", "", "", "", "", "Equal", "Unequal",
	"Less", "Greater", "LessEqual", "GreaterEqual", "Plus", "Minus", "Mul",
	"Div", "Mod", "Ampersand", "Negate", "Move", "Optional", "NilCoalescing",
	"Casting", "FailableCasting", "OpenParen", "CloseParen", "Transaction",
	"Struct", "Resource", "Contract", "Interface", "Fun", "Event", "Emit",
	"Pre", "Post", "Priv", "Auth", "Pub", "PubSet", "Return", "Break", "Continue",
	"Let", "Var", "If", "Else", "While", "True", "False", "Nil", "Import",
	"From", "Create", "Destroy", "Identifier", "DecimalLiteral", "BinaryLiteral",
	"OctalLiteral", "HexadecimalLiteral", "InvalidNumberLiteral", "StringLiteral",
	"WS", "Terminator", "BlockComment", "LineComment",
}

var lexerRuleNames = []string{
	"T__0", "T__1", "T__2", "T__3", "T__4", "T__5", "T__6", "T__7", "T__8",
	"T__9", "T__10", "T__11", "Equal", "Unequal", "Less", "Greater", "LessEqual",
	"GreaterEqual", "Plus", "Minus", "Mul", "Div", "Mod", "Ampersand", "Negate",
	"Move", "Optional", "NilCoalescing", "Casting", "FailableCasting", "OpenParen",
	"CloseParen", "Transaction", "Struct", "Resource", "Contract", "Interface",
	"Fun", "Event", "Emit", "Pre", "Post", "Priv", "Auth", "Pub", "PubSet",
	"Return", "Break", "Continue", "Let", "Var", "If", "Else", "While", "True",
	"False", "Nil", "Import", "From", "Create", "Destroy", "Identifier", "IdentifierHead",
	"IdentifierCharacter", "DecimalLiteral", "BinaryLiteral", "OctalLiteral",
	"HexadecimalLiteral", "InvalidNumberLiteral", "StringLiteral", "QuotedText",
	"EscapedCharacter", "HexadecimalDigit", "WS", "Terminator", "BlockComment",
	"LineComment",
}

type CadenceLexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

var lexerDecisionToDFA = make([]*antlr.DFA, len(lexerAtn.DecisionToState))

func init() {
	for index, ds := range lexerAtn.DecisionToState {
		lexerDecisionToDFA[index] = antlr.NewDFA(ds, index)
	}
}

func NewCadenceLexer(input antlr.CharStream) *CadenceLexer {

	l := new(CadenceLexer)

	l.BaseLexer = antlr.NewBaseLexer(input)
	l.Interpreter = antlr.NewLexerATNSimulator(l, lexerAtn, lexerDecisionToDFA, antlr.NewPredictionContextCache())

	l.channelNames = lexerChannelNames
	l.modeNames = lexerModeNames
	l.RuleNames = lexerRuleNames
	l.LiteralNames = lexerLiteralNames
	l.SymbolicNames = lexerSymbolicNames
	l.GrammarFileName = "Cadence.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// CadenceLexer tokens.
const (
	CadenceLexerT__0                 = 1
	CadenceLexerT__1                 = 2
	CadenceLexerT__2                 = 3
	CadenceLexerT__3                 = 4
	CadenceLexerT__4                 = 5
	CadenceLexerT__5                 = 6
	CadenceLexerT__6                 = 7
	CadenceLexerT__7                 = 8
	CadenceLexerT__8                 = 9
	CadenceLexerT__9                 = 10
	CadenceLexerT__10                = 11
	CadenceLexerT__11                = 12
	CadenceLexerEqual                = 13
	CadenceLexerUnequal              = 14
	CadenceLexerLess                 = 15
	CadenceLexerGreater              = 16
	CadenceLexerLessEqual            = 17
	CadenceLexerGreaterEqual         = 18
	CadenceLexerPlus                 = 19
	CadenceLexerMinus                = 20
	CadenceLexerMul                  = 21
	CadenceLexerDiv                  = 22
	CadenceLexerMod                  = 23
	CadenceLexerAmpersand            = 24
	CadenceLexerNegate               = 25
	CadenceLexerMove                 = 26
	CadenceLexerOptional             = 27
	CadenceLexerNilCoalescing        = 28
	CadenceLexerCasting              = 29
	CadenceLexerFailableCasting      = 30
	CadenceLexerOpenParen            = 31
	CadenceLexerCloseParen           = 32
	CadenceLexerTransaction          = 33
	CadenceLexerStruct               = 34
	CadenceLexerResource             = 35
	CadenceLexerContract             = 36
	CadenceLexerInterface            = 37
	CadenceLexerFun                  = 38
	CadenceLexerEvent                = 39
	CadenceLexerEmit                 = 40
	CadenceLexerPre                  = 41
	CadenceLexerPost                 = 42
	CadenceLexerPriv                 = 43
	CadenceLexerAuth                 = 44
	CadenceLexerPub                  = 45
	CadenceLexerPubSet               = 46
	CadenceLexerReturn               = 47
	CadenceLexerBreak                = 48
	CadenceLexerContinue             = 49
	CadenceLexerLet                  = 50
	CadenceLexerVar                  = 51
	CadenceLexerIf                   = 52
	CadenceLexerElse                 = 53
	CadenceLexerWhile                = 54
	CadenceLexerTrue                 = 55
	CadenceLexerFalse                = 56
	CadenceLexerNil                  = 57
	CadenceLexerImport               = 58
	CadenceLexerFrom                 = 59
	CadenceLexerCreate               = 60
	CadenceLexerDestroy              = 61
	CadenceLexerIdentifier           = 62
	CadenceLexerDecimalLiteral       = 63
	CadenceLexerBinaryLiteral        = 64
	CadenceLexerOctalLiteral         = 65
	CadenceLexerHexadecimalLiteral   = 66
	CadenceLexerInvalidNumberLiteral = 67
	CadenceLexerStringLiteral        = 68
	CadenceLexerWS                   = 69
	CadenceLexerTerminator           = 70
	CadenceLexerBlockComment         = 71
	CadenceLexerLineComment          = 72
)
