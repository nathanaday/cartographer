import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import App from "./App.vue";

describe("App", () => {
  it("mounts and shows the title", () => {
    const wrapper = mount(App);
    expect(wrapper.text()).toContain("Cartographer");
  });
});
