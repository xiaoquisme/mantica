import {
  Controller,
  Get,
  Post,
  Delete,
  Param,
  Body,
  Inject,
} from "@nestjs/common";
import { Hub } from "../hub/hub.js";

@Controller("api")
export class AppController {
  constructor(@Inject("HUB") private readonly hub: Hub) {}

  @Get("hub")
  getHub() {
    return {
      deviceId: this.hub.deviceId,
      url: this.hub.url,
      connectionState: this.hub.connectionState,
      agentCount: this.hub.listAgents().length,
    };
  }

  @Get("agents")
  listAgents() {
    return this.hub.listAgents().map((id) => {
      const agent = this.hub.getAgent(id);
      return { id, closed: agent?.closed ?? true };
    });
  }

  @Post("agents")
  createAgent(@Body() body?: { id?: string }) {
    const agent = this.hub.createAgent(body?.id);
    return { id: agent.id };
  }

  @Delete("agents/:id")
  deleteAgent(@Param("id") id: string) {
    const ok = this.hub.closeAgent(id);
    return { ok };
  }
}
