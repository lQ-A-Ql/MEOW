import { expect, test } from "bun:test"

import { buildMeowJSONArgs } from "../src/runner/meow"
import { buildVolArgs } from "../src/runner/vol"

test("buildMeowJSONArgs prepends the global JSON flag", () => {
  expect(buildMeowJSONArgs(["parse", "--banner-file", "banner.txt"])).toEqual([
    "--json",
    "parse",
    "--banner-file",
    "banner.txt",
  ])
})

test("buildVolArgs keeps Volatility invocation as argv pieces", () => {
  expect(
    buildVolArgs({
      memPath: "memory.raw",
      symbolsPath: "./symbols",
      plugin: "linux.pslist.PsList",
      extraArgs: ["--pid", "1"],
    }),
  ).toEqual(["-f", "memory.raw", "-s", "./symbols", "linux.pslist.PsList", "--pid", "1"])
})

test("buildVolArgs omits empty symbols path", () => {
  expect(
    buildVolArgs({
      memPath: "memory.raw",
      symbolsPath: " ",
      plugin: "banners.Banners",
    }),
  ).toEqual(["-f", "memory.raw", "banners.Banners"])
})
